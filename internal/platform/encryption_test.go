package platform

import (
	"errors"
	"testing"
)

// The gate that decides whether real candidate data may be stored, read on any
// machine — including the answers the acceptance laptop cannot produce without
// turning its own disk encryption off.
//
// The invariant that matters is one-directional: no failure, no silence, and no
// output the parser does not recognise may ever come back encrypted.
func TestTheEncryptionGateNeverReadsProtectionIntoSilence(t *testing.T) {
	broken := errors.New("exit status 1")

	for _, c := range []struct {
		name string
		out  string
		err  error
		want EncryptionStatus
	}{
		// manage-bde, as Windows actually prints it.
		{"protection on", "Conversion Status: Fully Encrypted\n    Protection Status: Protection On\n", nil, StatusEncrypted},
		{"protection off", "Conversion Status: Fully Decrypted\n    Protection Status: Protection Off\n", nil, StatusUnencrypted},
		{"decrypted without the protection line", "Conversion Status: Fully Decrypted\n", nil, StatusUnencrypted},
		{"mixed case", "protection status: PROTECTION ON", nil, StatusEncrypted},

		// The refusals.
		{"denied", "ERROR: Access is denied.", broken, StatusPermissionDenied},
		{"needs elevation", "This command requires elevation.", nil, StatusPermissionDenied},
		{"administrator", "You must be an administrator to run this.", nil, StatusPermissionDenied},

		// The absences. A machine with no manage-bde is the CIM fallback's
		// reason to exist, and it must not read as protected.
		{"tool missing", "", broken, StatusUnavailable},
		{"silent", "", nil, StatusUnavailable},
		{"whitespace", "   \n\t", nil, StatusUnavailable},
		{"unrecognised", "Estado de protección: activada", nil, StatusUnavailable},
		// "Protection Onwards" contains "protection on". Reading protection out
		// of it is the one mistake with consequences, so it answers unavailable.
		{"a longer word ending in it", "Protection Onwards, the disk is fine", nil, StatusUnavailable},
		{"the phrase at the very end", "Protection Status: Protection On", nil, StatusEncrypted},
		{"followed by punctuation", "Protection On.", nil, StatusEncrypted},
		{"a longer word then the real phrase", "Protection Onwards. Protection On\n", nil, StatusEncrypted},
	} {
		t.Run("manage-bde "+c.name, func(t *testing.T) {
			if got := readManageBDE(c.out, c.err); got != c.want {
				t.Fatalf("readManageBDE(%q) = %q, want %q", c.out, got, c.want)
			}
		})
	}

	for _, c := range []struct {
		name string
		out  string
		err  error
		want EncryptionStatus
	}{
		{"protected", "1\r\n", nil, StatusEncrypted},
		{"unprotected", "0\r\n", nil, StatusUnencrypted},
		{"unknown protection", "2", nil, StatusUnencrypted},
		{"no such volume", "", nil, StatusUnavailable},
		{"powershell failed", "", broken, StatusUnavailable},
		{"denied", "Access is denied", nil, StatusPermissionDenied},
		{"unexpected value", "3", nil, StatusUnavailable},
		{"a sentence", "Get-CimInstance : Invalid namespace", nil, StatusUnavailable},
	} {
		t.Run("cim "+c.name, func(t *testing.T) {
			if got := readCIM(c.out, c.err); got != c.want {
				t.Fatalf("readCIM(%q) = %q, want %q", c.out, got, c.want)
			}
		})
	}
}

// Stated as its own property, because it is the one that has consequences: real
// candidate data on an unencrypted disk.
func TestNoUnrecognisedOutputIsEverEncrypted(t *testing.T) {
	broken := errors.New("exit status 1")
	for _, out := range []string{
		"", "   ", "protection unknown", "Estado: activada", "ERROR", "null",
		"Conversion Status: Encryption in Progress", "3", "-1", "true", "yes",
		"Protection Onwards", "protection online", "protection onto the disk",
	} {
		for _, err := range []error{nil, broken} {
			if got := readManageBDE(out, err); got == StatusEncrypted {
				t.Fatalf("readManageBDE(%q, %v) read protection into it", out, err)
			}
			if got := readCIM(out, err); got == StatusEncrypted {
				t.Fatalf("readCIM(%q, %v) read protection into it", out, err)
			}
		}
	}
}
