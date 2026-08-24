package platform

import "errors"

// The two answers a credential store gives that a caller must act on, declared
// here rather than in either implementation so they can be compared against on
// any platform.
var (
	// ErrCredentialNotFound is no credential for that target. It is an ordinary
	// answer — "none stored" — not a fault.
	ErrCredentialNotFound = errors.New("credential not found")

	// ErrCredentialStoreUnsupported is a platform with nowhere a secret may go.
	//
	// There is deliberately no file or environment-variable fallback.
	ErrCredentialStoreUnsupported = errors.New("no supported credential store on this platform")
)
