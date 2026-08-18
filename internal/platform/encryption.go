package platform

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// EncryptionStatus is the volume-encryption gate result. No error path may ever
// yield StatusEncrypted — an unknown volume is never treated as protected.
type EncryptionStatus string

// The four answers the gate gives. They are separate values because what the
// recruiter does about each one differs.
const (
	StatusEncrypted        EncryptionStatus = "encrypted"
	StatusUnencrypted      EncryptionStatus = "unencrypted"
	StatusUnavailable      EncryptionStatus = "unavailable"
	StatusPermissionDenied EncryptionStatus = "permission-denied"
)

// denialOrEmpty maps tool failures to a status, or returns "" to keep parsing.
func denialOrEmpty(out string, err error) EncryptionStatus {
	low := strings.ToLower(out)
	if strings.Contains(low, "access is denied") ||
		strings.Contains(low, "requires elevation") ||
		strings.Contains(low, "administrator") {
		return StatusPermissionDenied
	}
	if err != nil || strings.TrimSpace(out) == "" {
		return StatusUnavailable
	}
	return ""
}

func runSystemTool(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// #nosec G204 -- fixed tool names; the only variable is a volume letter.
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}
