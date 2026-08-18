package platform

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EncryptionStatus is the volume-encryption gate result. No error path may ever
// yield StatusEncrypted — an unknown volume is never treated as protected.
type EncryptionStatus string

const (
	StatusEncrypted        EncryptionStatus = "encrypted"
	StatusUnencrypted      EncryptionStatus = "unencrypted"
	StatusUnavailable      EncryptionStatus = "unavailable"
	StatusPermissionDenied EncryptionStatus = "permission-denied"
)

// VolumeEncryption reports whether the volume holding path is protected by
// BitLocker or Windows Device Encryption. manage-bde is tried first; a CIM
// query is the fallback for SKUs where it is absent.
func VolumeEncryption(ctx context.Context, path string) EncryptionStatus {
	vol := filepath.VolumeName(path)
	if vol == "" {
		vol = "C:"
	}
	if s := manageBDEStatus(ctx, vol); s != StatusUnavailable {
		return s
	}
	return cimStatus(ctx, vol)
}

func manageBDEStatus(ctx context.Context, vol string) EncryptionStatus {
	out, err := runSystemTool(ctx, "manage-bde", "-status", vol, "-protectors")
	if s := denialOrEmpty(out, err); s != "" {
		return s
	}
	// Localized output is version-dependent; parse defensively and fall back to
	// the CIM query rather than guessing.
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "protection on"):
		return StatusEncrypted
	case strings.Contains(low, "protection off"), strings.Contains(low, "fully decrypted"):
		return StatusUnencrypted
	default:
		return StatusUnavailable
	}
}

func cimStatus(ctx context.Context, vol string) EncryptionStatus {
	// ponytail: PowerShell's CIM call instead of a COM/WMI dependency.
	query := `(Get-CimInstance -Namespace root/cimv2/security/microsoftvolumeencryption ` +
		`-ClassName Win32_EncryptableVolume -Filter "DriveLetter='` + vol + `'").ProtectionStatus`
	out, err := runSystemTool(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", query)
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
