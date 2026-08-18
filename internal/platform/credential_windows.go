package platform

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	credTargetPrefix        = "TalentHound:"
)

var (
	advapi32      = windows.NewLazySystemDLL("advapi32.dll")
	procCredWrite = advapi32.NewProc("CredWriteW")
	procCredRead  = advapi32.NewProc("CredReadW")
	procCredDel   = advapi32.NewProc("CredDeleteW")
	procCredFree  = advapi32.NewProc("CredFree")
)

// credential mirrors the Win32 CREDENTIALW struct.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// StoreSecret writes secret to Windows Credential Manager under
// "TalentHound:<purpose>". The secret value never appears in an error message.
func StoreSecret(purpose string, secret []byte) error {
	target, err := windows.UTF16PtrFromString(credTargetPrefix + purpose)
	if err != nil {
		return fmt.Errorf("encoding credential target: %w", err)
	}
	user, err := windows.UTF16PtrFromString("talent-hound")
	if err != nil {
		return fmt.Errorf("encoding credential user: %w", err)
	}
	cred := credential{
		Type:               credTypeGeneric,
		TargetName:         target,
		Persist:            credPersistLocalMachine,
		CredentialBlobSize: uint32(len(secret)),
		UserName:           user,
	}
	if len(secret) > 0 {
		cred.CredentialBlob = &secret[0]
	}
	ret, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if ret == 0 {
		return fmt.Errorf("writing credential %q: %w", purpose, callErr)
	}
	return nil
}

// LoadSecret reads the secret stored under "TalentHound:<purpose>".
func LoadSecret(purpose string) ([]byte, error) {
	target, err := windows.UTF16PtrFromString(credTargetPrefix + purpose)
	if err != nil {
		return nil, fmt.Errorf("encoding credential target: %w", err)
	}
	var pcred *credential
	ret, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(target)), credTypeGeneric, 0,
		uintptr(unsafe.Pointer(&pcred)))
	if ret == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return nil, fmt.Errorf("%w: %s", ErrCredentialNotFound, purpose)
		}
		return nil, fmt.Errorf("reading credential %q: %w", purpose, callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred))) //nolint:errcheck // frees only
	out := make([]byte, pcred.CredentialBlobSize)
	copy(out, unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize))
	return out, nil
}

// DeleteSecret removes the credential stored under "TalentHound:<purpose>".
func DeleteSecret(purpose string) error {
	target, err := windows.UTF16PtrFromString(credTargetPrefix + purpose)
	if err != nil {
		return fmt.Errorf("encoding credential target: %w", err)
	}
	ret, _, callErr := procCredDel.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if ret == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return fmt.Errorf("%w: %s", ErrCredentialNotFound, purpose)
		}
		return fmt.Errorf("deleting credential %q: %w", purpose, callErr)
	}
	return nil
}
