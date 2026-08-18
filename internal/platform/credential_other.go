//go:build !windows

package platform

import (
	"fmt"
	"runtime"
)

func unsupported(op string) error {
	return fmt.Errorf("%s a credential on %s: %w", op, runtime.GOOS, ErrCredentialStoreUnsupported)
}

// StoreSecret refuses: there is nowhere on this platform a secret may go.
func StoreSecret(_ string, _ []byte) error { return unsupported("storing") }

// LoadSecret refuses, for the same reason.
func LoadSecret(_ string) ([]byte, error) { return nil, unsupported("reading") }

// DeleteSecret refuses, for the same reason.
func DeleteSecret(_ string) error { return unsupported("removing") }
