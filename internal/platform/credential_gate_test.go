//go:build (windows && windowsgate) || credentialgate

package platform_test

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/platform"
)

func TestGateCredentialRoundTrip(t *testing.T) {
	purpose := fmt.Sprintf("gate-test-%d", os.Getpid())
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
	replacement := []byte("replacement-gate-value-do-not-log")
	if err := platform.StoreSecret(purpose, replacement); err != nil {
		t.Fatalf("replacing secret: %v", err)
	}
	got, err = platform.LoadSecret(purpose)
	if err != nil {
		t.Fatalf("loading replacement: %v", err)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("LoadSecret returned %q, want the replacement", got)
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
	if bytes.Contains(raw, replacement) {
		t.Fatal("replacement found in the database file")
	}
	if bytes.Contains(logs.Bytes(), secret) || bytes.Contains(logs.Bytes(), replacement) {
		t.Fatal("secret found in log output")
	}
}
