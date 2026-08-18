//go:build windowsgate

// External test package: an internal one would import internal/db, which
// imports this package, and Go refuses the cycle — which made the whole
// windowsgate build fail before it could run.
package platform_test

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"testing"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/platform"
)

func TestGateCredentialRoundTrip(t *testing.T) {
	const purpose = "gate-test"
	secret := []byte("s3cr3t-gate-value-do-not-log")

	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	if err := platform.StoreSecret(purpose, secret); err != nil {
		t.Fatalf("StoreSecret: %v", err)
	}
	t.Cleanup(func() { _ = platform.DeleteSecret(purpose) })

	got, err := platform.LoadSecret(purpose)
	if err != nil {
		t.Fatalf("LoadSecret: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("LoadSecret returned %q, want the stored secret", got)
	}
	if err := platform.DeleteSecret(purpose); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if _, err := platform.LoadSecret(purpose); !errors.Is(err, platform.ErrCredentialNotFound) {
		t.Fatalf("after delete got %v, want ErrCredentialNotFound", err)
	}

	// The secret must not reach the database file or the logs.
	dbPath := filepath.Join(t.TempDir(), "credgate.db")
	if _, err := db.Open(dbPath); err != nil {
		t.Fatalf("opening db: %v", err)
	}
	raw, err := os.ReadFile(dbPath) // #nosec G304 -- test-owned temp path
	if err != nil {
		t.Fatalf("reading db file: %v", err)
	}
	if bytes.Contains(raw, secret) {
		t.Fatal("secret found in the database file")
	}
	if bytes.Contains(logs.Bytes(), secret) {
		t.Fatal("secret found in log output")
	}
}
