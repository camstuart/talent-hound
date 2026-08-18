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
	// There is deliberately no fallback. "Provider keys live in the Windows
	// credential store and never enter application data" is a release gate, and
	// a development convenience that writes them to a file or an environment
	// variable is that gate failing quietly on a developer's machine first —
	// which is the machine where nobody is checking.
	ErrCredentialStoreUnsupported = errors.New("no supported credential store on this platform")
)
