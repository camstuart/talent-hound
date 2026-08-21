package main

import (
	"errors"
	"fmt"
	"slices"

	"camstuart/talent-hound/internal/platform"
)

// CredentialService holds provider secrets — and holds them nowhere this
// application can see. Every operation goes to the operating system's
// credential store; there is no column, no file, and no cache.
//
// Note what is absent: there is no Get. A getter on a service bound to the
// frontend is a getter reachable from the frontend, and from there a secret is
// one console log, one error message, or one crash report away from being
// written down somewhere it can be read. Go code that eventually needs the
// value reads it from the store at the moment of the request.
type CredentialService struct {
	store SecretStore
}

// SecretStore is the seam between this service's rules and the platform's
// store. It has exactly two implementations: the operating system's, and an
// in-memory one that exists only in tests. Nothing file-backed ships.
type SecretStore interface {
	Store(purpose string, secret []byte) error
	Load(purpose string) ([]byte, error)
	Delete(purpose string) error
}

// Providers are the external services a secret can be held for. A fixed list,
// because an arbitrary purpose string is an arbitrary credential-store entry.
var Providers = []string{"exa", "cloud"}

// NewCredentialService returns a service backed by the platform credential
// store. On a platform without one, every operation refuses — see
// platform.ErrCredentialStoreUnsupported for why there is no fallback.
func NewCredentialService() *CredentialService {
	return &CredentialService{store: platformStore{}}
}

// secret reads a stored value for Go code that has to make a request with it.
//
// Unexported on purpose, and that is the whole of the rule. The comment on this
// type says a getter bound to the frontend is a getter reachable from the
// frontend; Wails binds exported methods, so this one is reachable only from
// this package. It exists because "Go code that eventually needs the value
// reads it from the store at the moment of the request" needs a way to do that,
// and the alternative — handing the value to a caller through an exported
// method — is the thing being avoided.
//
// It returns a string because that is what an Authorization header wants, and
// keeping a []byte here would only look careful.
func (s *CredentialService) secret(provider string) (string, error) {
	if !slices.Contains(Providers, provider) {
		return "", fmt.Errorf("%q is not a provider this application stores a credential for", provider)
	}
	raw, err := s.store.Load(provider)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// platformStore is the shipping implementation: the operating system's.
type platformStore struct{}

func (platformStore) Store(purpose string, secret []byte) error {
	return platform.StoreSecret(purpose, secret)
}
func (platformStore) Load(purpose string) ([]byte, error) { return platform.LoadSecret(purpose) }
func (platformStore) Delete(purpose string) error         { return platform.DeleteSecret(purpose) }

// Store saves a provider's secret, replacing whatever was there.
//
// The secret is not trimmed or repaired: a key with a trailing space is a key
// the provider will reject, and silently fixing it hides the paste error rather
// than the recruiter seeing it fail once and looking again.
func (s *CredentialService) Store(provider, secret string) error {
	if err := checkProvider(provider); err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("the %s key must not be empty", provider)
	}
	if err := s.store.Store(provider, []byte(secret)); err != nil {
		// Deliberately not wrapped with anything derived from the secret.
		return fmt.Errorf("storing the %s key failed: %w", provider, redactStoreError(err))
	}
	return nil
}

// Has reports whether a secret is stored. It is the only thing the interface
// can learn about a credential.
func (s *CredentialService) Has(provider string) (bool, error) {
	if err := checkProvider(provider); err != nil {
		return false, err
	}
	secret, err := s.store.Load(provider)
	if errors.Is(err, platform.ErrCredentialNotFound) {
		// Not a fault: "none stored" is an ordinary answer to an ordinary
		// question, and making the recruiter interpret an error would be worse.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking for the %s key failed: %w", provider, redactStoreError(err))
	}
	return len(secret) > 0, nil
}

// List reports, for every provider, whether a secret is stored. It never
// reports a value, because there is no operation that could produce one.
func (s *CredentialService) List() (map[string]bool, error) {
	out := map[string]bool{}
	for _, p := range Providers {
		has, err := s.Has(p)
		if err != nil {
			return nil, err
		}
		out[p] = has
	}
	return out, nil
}

// Delete revokes a provider's credential. Local records are untouched:
// removing a credential disables the provider, it does not delete information.
func (s *CredentialService) Delete(provider string) error {
	if err := checkProvider(provider); err != nil {
		return err
	}
	err := s.store.Delete(provider)
	if err != nil && !errors.Is(err, platform.ErrCredentialNotFound) {
		return fmt.Errorf("removing the %s key failed: %w", provider, redactStoreError(err))
	}
	return nil
}

func checkProvider(provider string) error {
	if !slices.Contains(Providers, provider) {
		return fmt.Errorf("unknown provider %q", provider)
	}
	return nil
}

// errStoreRefused stands in for anything the credential store says that this
// service does not recognise.
var errStoreRefused = errors.New("the credential store refused the request")

// redactStoreError keeps a store's own error text out of everything this
// service returns. The two errors that mean something to a caller pass through;
// anything else becomes a fixed sentence.
//
// Today's Windows API does not echo the blob it was given, so this looks like
// caution about a thing that cannot happen. It is redaction by construction —
// the same rule Phase 6 applies to the sidecar's stderr — and it costs a
// less specific message in the one case where the store fails in a way nobody
// anticipated.
func redactStoreError(err error) error {
	switch {
	case errors.Is(err, platform.ErrCredentialStoreUnsupported):
		return platform.ErrCredentialStoreUnsupported
	case errors.Is(err, platform.ErrCredentialNotFound):
		return platform.ErrCredentialNotFound
	}
	return errStoreRefused
}
