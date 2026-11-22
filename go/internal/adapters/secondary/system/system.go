// Package system provides OS-backed Clock and IDGenerator adapters.
package system

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Clock is the wall clock.
type Clock struct{}

var _ ports.Clock = Clock{}

// Now implements ports.Clock.
func (Clock) Now() time.Time { return time.Now().UTC() }

// IDs mints random 128-bit hex ids prefixed with "inv_".
type IDs struct{}

var _ ports.IDGenerator = IDs{}

// NewID implements ports.IDGenerator.
func (IDs) NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "inv_" + hex.EncodeToString(b[:])
}
