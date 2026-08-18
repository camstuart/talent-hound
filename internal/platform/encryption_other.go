//go:build !windows && !darwin

package platform

import "context"

// VolumeEncryption cannot verify encryption away from the supported Windows
// laptop, and an unknown volume is never treated as protected — so real-data
// mode stays blocked here. There is deliberately no development override: a
// gate with a way past it is not a gate.
func VolumeEncryption(_ context.Context, _ string) EncryptionStatus {
	return StatusUnavailable
}
