package platform

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keychainService = "TalentHound"

// StoreSecret writes secret to macOS Keychain for the given purpose.
func StoreSecret(purpose string, secret []byte) error {
	if err := keyring.Set(keychainService, purpose, string(secret)); err != nil {
		return fmt.Errorf("writing credential %q: %w", purpose, err)
	}
	return nil
}

// LoadSecret reads the secret stored in macOS Keychain for the given purpose.
func LoadSecret(purpose string) ([]byte, error) {
	secret, err := keyring.Get(keychainService, purpose)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrCredentialNotFound, purpose)
	}
	if err != nil {
		return nil, fmt.Errorf("reading credential %q: %w", purpose, err)
	}
	return []byte(secret), nil
}

// DeleteSecret removes the secret stored in macOS Keychain for the given purpose.
func DeleteSecret(purpose string) error {
	if err := keyring.Delete(keychainService, purpose); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrCredentialNotFound, purpose)
		}
		return fmt.Errorf("deleting credential %q: %w", purpose, err)
	}
	return nil
}
