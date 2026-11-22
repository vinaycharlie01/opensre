// Package httpapi is the HTTP driving adapter: alert intake, investigation
// reads, and synchronous prompts.
package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// MaxBodyBytes bounds every request body.
const MaxBodyBytes = 1 << 20

// Deps are the use cases the server drives.
type Deps struct {
	Intake       ports.AlertIntake
	Investigator ports.Investigator
	Assistant    ports.Assistant
	// Tokens are accepted bearer tokens. When empty, only loopback callers
	// that are not browsers are allowed to use mutating endpoints.
	Tokens []string
	Log    *slog.Logger
}

// New returns the HTTP handler.
func New(d Deps) http.Handler {
	s := &server{d}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /alerts", s.guard(s.postAlert))
	mux.Handle("POST /v1/prompt", s.guard(s.postPrompt))
	mux.Handle("GET /investigations", s.guard(s.listInvestigations))
	mux.Handle("GET /investigations/{id}", s.guard(s.getInvestigation))
	return mux
}

type server struct{ d Deps }

// guard applies the shared trust boundary: a configured bearer token, or a
// local non-browser caller.
func (s *server) guard(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.d.Tokens) > 0 {
			if !tokenMatches(r.Header.Get("Authorization"), s.d.Tokens) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		} else if !isLoopback(r.RemoteAddr) {
			writeError(w, http.StatusForbidden, "set OPENSRE_GATEWAY_LISTENER_TOKEN to accept non-loopback callers")
			return
		} else if browserReachedLoopback(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		next(w, r)
	})
}

// tokenMatches compares against every accepted token without short-circuiting,
// so rejection time does not depend on which token or how much of it matched.
func tokenMatches(header string, tokens []string) bool {
	matched := 0
	for _, t := range tokens {
		matched |= subtle.ConstantTimeCompare([]byte(header), []byte("Bearer "+t))
	}
	return matched == 1
}

var loopbackHosts = map[string]bool{"127.0.0.1": true, "::1": true, "localhost": true}

func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return loopbackHosts[host]
}

// browserReachedLoopback detects a web page posting to localhost (foreign
// Origin) or a DNS-rebound page (foreign Host). Local tools such as curl send
// a loopback Host and no Origin.
func browserReachedLoopback(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" && !loopbackHosts[urlHost(origin)] {
		return true
	}
	return !loopbackHosts[urlHost(r.Host)]
}

func urlHost(v string) string {
	if !strings.Contains(v, "//") {
		v = "//" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

type alertPayload struct {
	Text       string     `json:"text"`
	AlertName  string     `json:"alert_name"`
	Severity   string     `json:"severity"`
	Source     string     `json:"source"`
	ReceivedAt *time.Time `json:"received_at"`
}

func (s *server) postAlert(w http.ResponseWriter, r *http.Request) {
	var p alertPayload
	if err := decodeStrict(r.Body, &p); err != nil {
		s.d.Log.WarnContext(r.Context(), "alert payload rejected", "err", err)
		writeError(w, http.StatusBadRequest, "invalid alert payload")
		return
	}
	a := alert.Alert{Text: p.Text, Name: p.AlertName, Severity: alert.Severity(p.Severity), Source: p.Source}
	if p.ReceivedAt != nil {
		a.ReceivedAt = *p.ReceivedAt
	}
	receipt, err := s.d.Intake.Accept(r.Context(), a)
	switch {
	case errors.Is(err, alert.ErrEmptyText), errors.Is(err, alert.ErrTextTooLong):
		writeError(w, http.StatusBadRequest, "invalid alert payload: "+err.Error())
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	body := map[string]any{"queued": true, "queue_depth": receipt.QueueDepth}
	if receipt.Evicted {
		body["warning"] = "inbox full, oldest alert dropped"
	}
	writeJSON(w, http.StatusAccepted, body)
}

func (s *server) postPrompt(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Question string `json:"question"`
	}
	if err := decodeStrict(r.Body, &p); err != nil || strings.TrimSpace(p.Question) == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	ans, err := s.d.Assistant.Ask(r.Context(), p.Question)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"answer": ans.Text, "tool_calls": ans.ToolCalls})
}

func (s *server) listInvestigations(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			writeError(w, http.StatusBadRequest, "limit must be 1-500")
			return
		}
		limit = n
	}
	list, err := s.d.Investigator.List(r.Context(), limit)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"investigations": list})
}

func (s *server) getInvestigation(w http.ResponseWriter, r *http.Request) {
	inv, err := s.d.Investigator.Get(r.Context(), investigation.ID(r.PathValue("id")))
	if errors.Is(err, ports.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

// internalError logs full detail server-side and returns a generic message:
// provider and model internals must not reach external callers.
func (s *server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.d.Log.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func decodeStrict(body io.Reader, v any) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data after JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
