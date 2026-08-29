package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// No real provider key appears anywhere in this repository. The value below is
// invented and exists only to be searched for in places it must never reach.
const inventedKey = "not-a-real-key-QQQZZZ-9f2b41d7c8e5"

// memoryStore is the test-only SecretStore. It is the only implementation
// besides the operating system's, and it deliberately does not ship: a
// file-backed store on a developer's machine is the PRD's credential gate
// failing quietly where nobody is looking.
type memoryStore struct {
	mu       sync.Mutex
	secrets  map[string][]byte
	failWith error
}

func newMemoryStore() *memoryStore { return &memoryStore{secrets: map[string][]byte{}} }

func (m *memoryStore) Store(purpose string, secret []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failWith != nil {
		return m.failWith
	}
	m.secrets[purpose] = append([]byte(nil), secret...)
	return nil
}

func (m *memoryStore) Load(purpose string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failWith != nil {
		return nil, m.failWith
	}
	secret, ok := m.secrets[purpose]
	if !ok {
		return nil, platform.ErrCredentialNotFound
	}
	return secret, nil
}

func (m *memoryStore) Delete(purpose string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failWith != nil {
		return m.failWith
	}
	if _, ok := m.secrets[purpose]; !ok {
		return platform.ErrCredentialNotFound
	}
	delete(m.secrets, purpose)
	return nil
}

func newCredentialService() (*CredentialService, *memoryStore) {
	store := newMemoryStore()
	return &CredentialService{store: store}, store
}

func TestCredentialLifecycle(t *testing.T) {
	svc, store := newCredentialService()

	has, err := svc.Has("exa")
	if err != nil || has {
		t.Fatalf("a provider with no credential reported has=%v err=%v", has, err)
	}

	if err := svc.Store("exa", inventedKey); err != nil {
		t.Fatal(err)
	}
	if has, err := svc.Has("exa"); err != nil || !has {
		t.Fatalf("a stored credential reported has=%v err=%v", has, err)
	}

	// Replacing keeps only the new value.
	if err := svc.Store("exa", inventedKey+"-second"); err != nil {
		t.Fatal(err)
	}
	if got := string(store.secrets["exa"]); got != inventedKey+"-second" {
		t.Fatalf("the store holds %q after a replacement", got)
	}

	if err := svc.Delete("exa"); err != nil {
		t.Fatal(err)
	}
	if has, _ := svc.Has("exa"); has {
		t.Fatal("a revoked credential still reports as stored")
	}
	// Revoking one that is not there is not a fault: the caller asked for a
	// state that already holds.
	if err := svc.Delete("exa"); err != nil {
		t.Fatalf("revoking a missing credential failed: %v", err)
	}
}

func TestCredentialServiceRefusesWhatItMust(t *testing.T) {
	svc, _ := newCredentialService()
	if err := svc.Store("exa", ""); err == nil {
		t.Error("an empty secret was stored")
	}
	if err := svc.Store("nowhere", inventedKey); err == nil {
		t.Error("an unknown provider was accepted")
	}
	if _, err := svc.Has("nowhere"); err == nil {
		t.Error("an unknown provider was queried")
	}
}

func TestThereIsNoWayToReadASecretBack(t *testing.T) {
	// The absence of a getter is the design, so it is asserted rather than left
	// to be noticed: List reports existence and nothing else.
	svc, _ := newCredentialService()
	if err := svc.Store("exa", inventedKey); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if !stored["exa"] || stored["cloud"] {
		t.Fatalf("List reported %+v", stored)
	}
	for provider, has := range stored {
		if strings.Contains(provider, inventedKey) {
			t.Fatal("List leaked a secret through a key")
		}
		_ = has
	}
}

func TestAStoreFailureDoesNotQuoteTheSecret(t *testing.T) {
	svc, store := newCredentialService()
	// A store that echoes what it was given — the failure mode the redaction
	// exists for, whether or not any real store behaves this way.
	store.failWith = errors.New("write failed for value " + inventedKey)

	err := svc.Store("exa", inventedKey)
	if err == nil {
		t.Fatal("the store failure was not reported")
	}
	if strings.Contains(err.Error(), inventedKey) {
		t.Fatalf("the error quoted the secret: %q", err)
	}
	if !strings.Contains(err.Error(), "exa") {
		t.Errorf("the error does not name the provider: %q", err)
	}
}

func TestASecretReachesNoDatabaseNoLogAndNoError(t *testing.T) {
	// Crude on purpose: the test does not care how a secret might have escaped,
	// only whether it did. Asserting that no code path logs one would be
	// asserting about code that has not been written yet.
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "credentials.db")
	gdb, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, gdb)
	svc, _ := newCredentialService()

	var errorText strings.Builder
	record := func(err error) {
		if err != nil {
			errorText.WriteString(err.Error())
			errorText.WriteString("\n")
		}
	}
	// Every operation, including the ones that fail.
	record(svc.Store("exa", inventedKey))
	record(svc.Store("cloud", inventedKey))
	record(svc.Store("nowhere", inventedKey))
	record(svc.Store("exa", ""))
	_, err = svc.Has("exa")
	record(err)
	_, err = svc.List()
	record(err)
	record(svc.Delete("cloud"))

	// Something unrelated writes to the database, so the file is not empty.
	if err := gdb.Create(&models.Initiative{Name: "Anything", Type: models.InitiativeTypeTalentSearch}).Error; err != nil {
		t.Fatal(err)
	}
	if sql, err := gdb.DB(); err == nil {
		_ = sql.Close()
	}

	// The whole data folder, not only the database: a recovery copy is a copy
	// of the folder.
	found := []string{}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path) // #nosec G304 -- a temp dir this test made
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(inventedKey)) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Errorf("the secret is in the data folder: %v", found)
	}
	if strings.Contains(logs.String(), inventedKey) {
		t.Errorf("the secret is in the logs: %q", logs.String())
	}
	if strings.Contains(errorText.String(), inventedKey) {
		t.Errorf("the secret is in an error: %q", errorText.String())
	}
}

func TestThePlatformStoreRefusesWhereThereIsNoStore(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("this platform has a credential store; the round trip is a gate test")
	}
	svc := NewCredentialService()
	err := svc.Store("exa", inventedKey)
	if !errors.Is(err, platform.ErrCredentialStoreUnsupported) {
		t.Fatalf("storing on %s returned %v, want the unsupported-platform error", runtime.GOOS, err)
	}
	// And nothing anywhere pretends it worked.
	if has, err := svc.Has("exa"); has || err == nil {
		t.Fatalf("Has reported %v (err %v) on a platform with no store", has, err)
	}
}

// The service's exported surface is exactly four methods, and none of them
// hands back a secret.
//
// The absence of a getter is the design, and until now it was asserted by
// calling what exists. That catches a getter nobody adds. This catches the one
// somebody adds: Wails binds exported methods, so an exported reader is a
// reader reachable from the frontend, and from there a secret is one console
// log or one crash report away from being written down.
//
// Go code that needs the value reads it through the unexported reader, at the
// moment of the request. That is deliberately not on this list.
func TestTheExportedSurfaceCannotHandBackASecret(t *testing.T) {
	allowed := map[string]bool{"Store": true, "Has": true, "List": true, "Delete": true}

	surface := reflect.TypeOf(&CredentialService{})
	seen := map[string]bool{}
	for i := 0; i < surface.NumMethod(); i++ {
		name := surface.Method(i).Name
		seen[name] = true
		if !allowed[name] {
			t.Errorf("CredentialService exports %q, which the frontend can call — "+
				"if it can return a secret, the store's whole point is gone", name)
		}
	}
	for name := range allowed {
		if !seen[name] {
			t.Errorf("%q is gone, and this test is now guarding a surface that moved", name)
		}
	}
}
