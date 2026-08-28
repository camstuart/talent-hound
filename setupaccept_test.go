package main

import (
	"testing"

	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/setup"
)

// The choice to hold data without encryption is a setting: it permits the
// write, survives a restart, is shown as a warning rather than hidden, and can
// be withdrawn.
func TestAcceptingUnencryptedStorageIsARecordedChoice(t *testing.T) {
	e := newSetupEnv(t)
	e.encrypted(platform.StatusUnavailable)
	if err := e.setup.AllowRealData(); err == nil {
		t.Fatal("an uncheckable volume permitted data before any choice")
	}
	if err := e.setup.AcceptUnencrypted(true); err != nil {
		t.Fatalf("accepting: %v", err)
	}
	if err := e.setup.AllowRealData(); err != nil {
		t.Fatalf("accepted storage still refused: %v", err)
	}
	st := e.state(t)
	if !st.RealData || !st.UnencryptedAccepted || st.Warning == "" || st.RealDataWhy != "" {
		t.Fatalf("state = realData %v accepted %v warning %q why %q", st.RealData, st.UnencryptedAccepted, st.Warning, st.RealDataWhy)
	}
	if st.Next == setup.StepEncryption {
		t.Fatal("the wizard is still on the encryption step")
	}
	scope := e.setup.Scope()
	if !scope.RealData || scope.Warning == "" {
		t.Fatalf("scope state = %+v", scope)
	}

	// Survives a restart.
	again, err := NewSetupService(e.db, e.models, e.setup.confDir, e.setup.dataDir)
	if err != nil {
		t.Fatalf("restarting: %v", err)
	}
	again.mu.Lock()
	again.encryption = platform.StatusUnencrypted
	again.mu.Unlock()
	if err := again.AllowRealData(); err != nil {
		t.Fatalf("the acceptance did not survive a restart: %v", err)
	}

	// And can be withdrawn.
	if err := e.setup.AcceptUnencrypted(false); err != nil {
		t.Fatalf("withdrawing: %v", err)
	}
	if err := e.setup.AllowRealData(); err == nil {
		t.Fatal("withdrawn acceptance still permitted data")
	}
}
