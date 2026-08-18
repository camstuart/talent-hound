//go:build windowsgate

package platform

import (
	"context"
	"testing"
)

// TestGateVolumeEncryption records the status of the real data volume. It fails
// only when the status cannot be determined at all — encrypted vs unencrypted is
// evidence for the record, not a pass/fail condition here.
func TestGateVolumeEncryption(t *testing.T) {
	got := VolumeEncryption(context.Background(), `C:\`)
	t.Logf("EVIDENCE volume-encryption C: = %s", got)
	switch got {
	case StatusEncrypted, StatusUnencrypted:
	case StatusUnavailable, StatusPermissionDenied:
		t.Fatalf("encryption status for C: is %s — the gate cannot rely on it", got)
	default:
		t.Fatalf("unknown status %q", got)
	}
}

// TestGateVolumeEncryptionNeverGuessesEncrypted proves the failure paths return
// distinct, non-encrypted results.
func TestGateVolumeEncryptionNeverGuessesEncrypted(t *testing.T) {
	// A volume that does not exist stands in for "status unavailable".
	if got := VolumeEncryption(context.Background(), `Q:\nope`); got == StatusEncrypted {
		t.Fatalf("missing volume reported as encrypted")
	} else {
		t.Logf("EVIDENCE volume-encryption missing-volume = %s", got)
	}

	cases := map[string]EncryptionStatus{
		"ERROR: Access is denied.":                         StatusPermissionDenied,
		"This command requires elevation.":                 StatusPermissionDenied,
		"You must be an administrator to run this action.": StatusPermissionDenied,
		"": StatusUnavailable,
	}
	for out, want := range cases {
		if got := denialOrEmpty(out, nil); got != want {
			t.Errorf("denialOrEmpty(%q) = %s, want %s", out, got, want)
		}
	}
}
