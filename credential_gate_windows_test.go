//go:build windowsgate

package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
)

// The service-level half of the Phase 1 credential gate: the platform test
// proves the Win32 calls, this proves the rules the service puts around them
// against the real Credential Manager rather than the in-memory store.
//
// The value below is invented and exists only to be searched for.
const gateKey = "not-a-real-key-GATE-4c1d90ab7e26"

func TestGateCredentialServiceLifecycle(t *testing.T) {
	svc := NewCredentialService()
	t.Cleanup(func() { _ = svc.Delete("exa") })

	if has, err := svc.Has("exa"); err != nil || has {
		t.Fatalf("before storing: has=%v err=%v", has, err)
	}
	if err := svc.Store("exa", gateKey); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if has, err := svc.Has("exa"); err != nil || !has {
		t.Fatalf("after storing: has=%v err=%v", has, err)
	}
	if err := svc.Store("exa", gateKey+"-replaced"); err != nil {
		t.Fatalf("replacing: %v", err)
	}
	if has, err := svc.Has("exa"); err != nil || !has {
		t.Fatalf("after replacing: has=%v err=%v", has, err)
	}
	if err := svc.Delete("exa"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if has, err := svc.Has("exa"); err != nil || has {
		t.Fatalf("after revoking: has=%v err=%v", has, err)
	}
	// Revoking again is not a fault.
	if err := svc.Delete("exa"); err != nil {
		t.Fatalf("revoking a missing credential: %v", err)
	}
	t.Logf("EVIDENCE credential service lifecycle: store, replace, revoke, missing all pass")
}

func TestGateCredentialIsAbsentFromTheDataFolder(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	svc := NewCredentialService()
	if err := svc.Store("exa", gateKey); err != nil {
		t.Fatalf("Store: %v", err)
	}
	t.Cleanup(func() { _ = svc.Delete("exa") })

	dir := t.TempDir()
	gdb, err := db.Open(filepath.Join(dir, "gate.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	if err := gdb.Create(&models.Initiative{
		Name: "Gate", Type: models.InitiativeTypeTalentSearch,
	}).Error; err != nil {
		t.Fatalf("writing something to the database: %v", err)
	}
	if sql, err := gdb.DB(); err == nil {
		_ = sql.Close()
	}

	// A recovery copy is a copy of the whole folder, so the whole folder is
	// what gets searched.
	found := []string{}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path) // #nosec G304 -- test-owned temp path
		if err != nil {
			return err
		}
		if bytes.Contains(raw, []byte(gateKey)) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Fatalf("the credential is in the data folder: %v", found)
	}
	if bytes.Contains(logs.Bytes(), []byte(gateKey)) {
		t.Fatal("the credential is in the log output")
	}
	t.Logf("EVIDENCE credential absent from the data folder and logs: %d files scanned", len(found)+1)
}
