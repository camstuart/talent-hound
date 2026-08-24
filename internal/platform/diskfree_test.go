package platform

import "testing"

func TestFreeDiskBytesOfARealFolderIsPositive(t *testing.T) {
	free, err := FreeDiskBytes(t.TempDir())
	if err != nil {
		t.Fatalf("free space of a real folder: %v", err)
	}
	if free <= 0 {
		t.Fatalf("free space of a real folder should be positive, got %d", free)
	}
}

func TestFreeDiskBytesOfAMissingFolderFails(t *testing.T) {
	if _, err := FreeDiskBytes("/no/such/folder/anywhere"); err == nil {
		t.Fatal("a missing folder has no answerable free space")
	}
}
