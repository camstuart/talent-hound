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

// readManageBDE interprets `manage-bde -status`, and readCIM interprets the
// Win32_EncryptableVolume ProtectionStatus. Both live here rather than beside
// the call that produces them, because they are the part that decides whether
// real candidate data may be stored and the part a development machine can
// exercise. On Windows the gate can only ever report what that machine's own
// volume happens to be: a laptop with BitLocker on cannot produce "protection
// off", which is the answer with consequences.
//
// Localized output is version-dependent, so both parse defensively and answer
// unavailable rather than guess. No path here returns encrypted from anything
// but an explicit statement that it is.
func readManageBDE(out string, err error) EncryptionStatus {
	if s := denialOrEmpty(out, err); s != "" {
		return s
	}
	low := strings.ToLower(out)
	switch {
	case saysWord(low, "protection on"):
		return StatusEncrypted
	case saysWord(low, "protection off"), saysWord(low, "fully decrypted"):
		return StatusUnencrypted
	default:
		return StatusUnavailable
	}
}

// saysWord reports whether text states the phrase and not a longer word ending
// in it. "Protection Onwards" contains "protection on", and reading protection
// out of it is the one mistake here with consequences — real candidate data on
// a disk nobody encrypted. Unrecognised output answers unavailable instead,
// which costs a recruiter an explanation and costs nobody their data.
func saysWord(text, phrase string) bool {
	for at := 0; ; {
		i := strings.Index(text[at:], phrase)
		if i < 0 {
			return false
		}
		end := at + i + len(phrase)
		if end == len(text) || !isWordByte(text[end]) {
			return true
		}
		at = end
	}
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func readCIM(out string, err error) EncryptionStatus {
	if s := denialOrEmpty(out, err); s != "" {
		return s
	}
	switch strings.TrimSpace(out) {
	case "1":
		return StatusEncrypted
	case "0", "2":
		return StatusUnencrypted
	default:
		return StatusUnavailable
	}
}

// readShellProtection interprets the Shell property System.Volume.BitLockerProtection,
// which is readable without elevation when manage-bde and CIM are not. Its
// values: 1 on, 2 off, 3 encrypting, 4 decrypting, 5 suspended, 6 on and
// locked. Only fully on counts as encrypted; a volume still encrypting, or
// suspended, is one an attacker can read. 0 is what a machine with no
// BitLocker at all reports, and is unencrypted for the same reason.
func readShellProtection(out string, err error) EncryptionStatus {
	if s := denialOrEmpty(out, err); s != "" {
		return s
	}
	switch strings.TrimSpace(out) {
	case "1", "6":
		return StatusEncrypted
	case "0", "2", "3", "4", "5":
		return StatusUnencrypted
	default:
		return StatusUnavailable
	}
}

func runSystemTool(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// #nosec G204 -- fixed tool names; the only variable is a volume letter.
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}
