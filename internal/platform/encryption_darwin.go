//go:build darwin

package platform

import (
	"context"
	"strings"
)

// VolumeEncryption reports FileVault's state. macOS is a development platform
// for this product, not a shipping one — but the gate is a real check here
// rather than a bypass, because a development build that cannot evaluate the
// gate is a development build where the gate is never exercised.
func VolumeEncryption(ctx context.Context, _ string) EncryptionStatus {
	out, err := runSystemTool(ctx, "fdesetup", "status")
	if s := denialOrEmpty(out, err); s != "" {
		return s
	}
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "filevault is on"):
		return StatusEncrypted
	case strings.Contains(low, "filevault is off"):
		return StatusUnencrypted
	default:
		return StatusUnavailable
	}
}
