package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// Every fixture here is invented or generated. No real candidate information
// enters this repository, its logs, or its test output.

func newArtifactService(t *testing.T) *ArtifactService {
	t.Helper()
	return NewArtifactService(newTestDB(t))
}

// anInitiative creates a target to link artifacts to.
func anInitiative(t *testing.T, gdb *gorm.DB, name string) uint {
	t.Helper()
	created, err := NewInitiativeService(gdb).Create(name, models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating initiative: %v", err)
	}
	return created.ID
}

// The first bytes of the smallest documents the app accepts. They are headers,
// not real documents: this phase stores bytes and never opens them.
var (
	pdfBytes  = []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\ntrailer\n%%EOF\n")
	docxBytes = append([]byte("PK\x03\x04\x14\x00\x06\x00"), bytes.Repeat([]byte{0}, 40)...)
)

func TestArtifactBytesRoundTripExactly(t *testing.T) {
	s := newArtifactService(t)

	binary := make([]byte, 4096)
	if _, err := rand.Read(binary); err != nil {
		t.Fatalf("generating random bytes: %v", err)
	}

	cases := []struct {
		name      string
		filename  string
		data      []byte
		mediaType string
	}{
		{name: "plain text", filename: "notes.txt", data: []byte("A plain note.\nWith two lines.\n"), mediaType: "text/plain; charset=utf-8"},
		{name: "markdown", filename: "notes.md", data: []byte("# Heading\n\n- a\n- b\n"), mediaType: "text/markdown"},
		{name: "pdf", filename: "resume.pdf", data: pdfBytes, mediaType: "application/pdf"},
		{name: "docx", filename: "resume.docx", data: docxBytes, mediaType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{name: "arbitrary binary", filename: "blob.bin", data: binary, mediaType: "application/octet-stream"},
		{name: "unicode text", filename: "notes.txt", data: []byte("Zoë Ólafsdóttir-李 🇮🇸\n"), mediaType: "text/plain; charset=utf-8"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := s.create("", tc.filename, "unit test", tc.data, "", 0)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			got, err := s.bytes(created.ID)
			if err != nil {
				t.Fatalf("bytes: %v", err)
			}
			if !bytes.Equal(got, tc.data) {
				t.Errorf("bytes changed in storage: got %d bytes, want %d", len(got), len(tc.data))
			}
			if created.MediaType != tc.mediaType {
				t.Errorf("media type is %q, want %q", created.MediaType, tc.mediaType)
			}
			if created.ByteLength != int64(len(tc.data)) {
				t.Errorf("byte length is %d, want %d", created.ByteLength, len(tc.data))
			}
			sum := sha256.Sum256(tc.data)
			if created.SHA256 != hex.EncodeToString(sum[:]) {
				t.Errorf("sha256 is %q, want the hash of the submitted bytes", created.SHA256)
			}
			if created.CapturedAt.IsZero() {
				t.Error("capture time was not set")
			}
			if created.DisplayName != tc.filename {
				t.Errorf("display name defaulted to %q, want the filename", created.DisplayName)
			}
		})
	}
}

func TestArtifactCreateThroughTheBase64Boundary(t *testing.T) {
	s := newArtifactService(t)
	data := []byte("bytes that crossed a JSON boundary\x00\x01\x02")

	created, err := s.Create(ArtifactInput{
		OriginalFilename: "notes.txt",
		Source:           "unit test",
		DataBase64:       base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	encoded, err := s.Bytes(created.ID)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	got, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decoding returned bytes: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("bytes did not survive the base64 round trip")
	}

	if _, err := s.Create(ArtifactInput{OriginalFilename: "x.txt", DataBase64: "not base64!"}); err == nil {
		t.Error("Create accepted invalid base64")
	}
}

func TestIdenticalBytesCreateTwoArtifacts(t *testing.T) {
	gdb := newTestDB(t)
	s := NewArtifactService(gdb)
	data := []byte("the very same bytes")

	first, err := s.create("", "from-email.txt", "Emailed by the candidate", data, models.LinkInitiative, anInitiative(t, gdb, "One"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := s.create("", "from-portal.txt", "Downloaded from the portal", data, models.LinkInitiative, anInitiative(t, gdb, "Two"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if first.ID == second.ID {
		t.Fatal("identical bytes were deduplicated into one artifact")
	}
	if first.SHA256 != second.SHA256 {
		t.Error("identical bytes hashed differently")
	}
	if first.OriginalFilename == second.OriginalFilename || first.Source == second.Source {
		t.Error("the two ingestions did not keep independent provenance")
	}
	for _, id := range []uint{first.ID, second.ID} {
		got, err := s.bytes(id)
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("artifact %d lost its bytes: %v", id, err)
		}
	}
}

func TestArtifactSizeBoundaries(t *testing.T) {
	s := newArtifactService(t)

	empty, err := s.create("Pasted note", "", "pasted", nil, "", 0)
	if err != nil {
		t.Fatalf("zero bytes was refused: %v", err)
	}
	if empty.ByteLength != 0 {
		t.Errorf("empty artifact has length %d", empty.ByteLength)
	}
	sum := sha256.Sum256(nil)
	if empty.SHA256 != hex.EncodeToString(sum[:]) {
		t.Error("empty artifact does not carry the hash of empty input")
	}

	atLimit := bytes.Repeat([]byte{'a'}, models.MaxArtifactBytes)
	got, err := s.create("", "big.txt", "", atLimit, "", 0)
	if err != nil {
		t.Fatalf("exactly at the limit was refused: %v", err)
	}
	if got.ByteLength != models.MaxArtifactBytes {
		t.Errorf("stored %d bytes, want the limit", got.ByteLength)
	}

	// A fresh slice, not append(atLimit, ...): that would share atLimit's array.
	over := bytes.Repeat([]byte{'a'}, models.MaxArtifactBytes+1)
	_, err = s.create("", "too-big.txt", "", over, "", 0)
	if err == nil {
		t.Fatal("one byte over the limit was accepted")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("error %q does not name the limit", err)
	}
	all, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("the refused ingestion was persisted: %d artifacts", len(all))
	}
}

func TestArtifactMetadataIsHandledSafely(t *testing.T) {
	s := newArtifactService(t)

	// Extension and content disagree: the bytes win, and it is not an error.
	mislabelled, err := s.create("", "resume.pdf", "", []byte("this is really just text\n"), "", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(mislabelled.MediaType, "text/plain") {
		t.Errorf("media type is %q, want the type detected from the bytes", mislabelled.MediaType)
	}
	if mislabelled.OriginalFilename != "resume.pdf" {
		t.Errorf("original filename was rewritten to %q", mislabelled.OriginalFilename)
	}

	// A path-like filename is provenance, not an instruction: stored verbatim,
	// never used to open anything.
	pathLike := "../../etc/passwd"
	traversal, err := s.create("", pathLike, "", []byte("x traversal"), "", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if traversal.OriginalFilename != pathLike {
		t.Errorf("path-like filename was sanitised to %q, losing provenance", traversal.OriginalFilename)
	}

	unicodeName := "履歴書 – Zoë.txt"
	unicode, err := s.create("", unicodeName, "", []byte("x unicode"), "", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if unicode.OriginalFilename != unicodeName {
		t.Errorf("unicode filename was mangled to %q", unicode.OriginalFilename)
	}

	// No filename at all — pasted text — needs a display name instead.
	if _, err := s.create("", "", "pasted", []byte("x unnamed"), "", 0); err == nil {
		t.Error("an artifact with neither filename nor display name was accepted")
	}
	pasted, err := s.create("  Pasted from an email  ", "", "pasted", []byte("x pasted"), "", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if pasted.OriginalFilename != "" || pasted.DisplayName != "Pasted from an email" {
		t.Errorf("pasted artifact is %+v", pasted)
	}

	// Display names are labels: duplicates are fine when the bytes differ.
	for i := range 2 {
		if _, err := s.create("Same label", "a.txt", "", []byte{'x', byte('0' + i)}, "", 0); err != nil {
			t.Fatalf("duplicate display name was refused: %v", err)
		}
	}
}

func TestRenameChangesOnlyTheDisplayName(t *testing.T) {
	s := newArtifactService(t)
	data := []byte("evidence")
	created, err := s.create("", "resume.pdf", "Emailed by the candidate", data, "", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	renamed, err := s.Rename(created.ID, "  Priya — resume  ")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.DisplayName != "Priya — resume" {
		t.Errorf("display name is %q", renamed.DisplayName)
	}

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.OriginalFilename != created.OriginalFilename || got.MediaType != created.MediaType ||
		got.ByteLength != created.ByteLength || got.SHA256 != created.SHA256 ||
		got.Source != created.Source || !got.CapturedAt.Equal(created.CapturedAt) {
		t.Errorf("renaming changed provenance:\n got %+v\nwant %+v", got, created)
	}
	stored, err := s.bytes(created.ID)
	if err != nil || !bytes.Equal(stored, data) {
		t.Errorf("renaming changed the bytes: %v", err)
	}

	if _, err := s.Rename(created.ID, "   "); err == nil {
		t.Error("Rename accepted a blank display name")
	}
	if after, _ := s.Get(created.ID); after.DisplayName != "Priya — resume" {
		t.Errorf("rejected rename changed the stored name to %q", after.DisplayName)
	}
	if _, err := s.Rename(9999, "Nowhere"); err == nil {
		t.Error("Rename accepted an unknown artifact")
	}
}

func TestArtifactLinkingAndDetaching(t *testing.T) {
	gdb := newTestDB(t)
	s := NewArtifactService(gdb)
	records := NewRecordService(gdb)
	initiativeID := anInitiative(t, gdb, "Records")
	candidate, err := records.CreateCandidate(models.Candidate{FullName: "Priya Raman"})
	if err != nil {
		t.Fatalf("CreateCandidate: %v", err)
	}

	data := []byte("one set of bytes, two homes")
	artifact, err := s.create("", "resume.pdf", "", data, models.LinkInitiative, initiativeID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Link(artifact.ID, models.LinkCandidate, candidate.ID); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Both targets resolve the same artifact, and the bytes were not copied.
	for _, target := range []struct {
		typ models.LinkTarget
		id  uint
	}{{models.LinkInitiative, initiativeID}, {models.LinkCandidate, candidate.ID}} {
		listed, err := s.ListForTarget(target.typ, target.id)
		if err != nil {
			t.Fatalf("ListForTarget(%s): %v", target.typ, err)
		}
		if len(listed) != 1 || listed[0].ID != artifact.ID {
			t.Fatalf("%s resolved %+v, want the one artifact", target.typ, listed)
		}
	}
	all, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("linking copied the artifact: %d rows", len(all))
	}
	if len(all[0].Bytes) != 0 {
		t.Error("listings carry the blob; they should not")
	}

	// Linking again is a no-op, not a failure.
	if err := s.Link(artifact.ID, models.LinkCandidate, candidate.ID); err != nil {
		t.Fatalf("re-linking failed: %v", err)
	}
	links, err := s.Links(artifact.ID)
	if err != nil {
		t.Fatalf("Links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("artifact has %d links, want 2", len(links))
	}

	// Unknown targets are refused.
	if err := s.Link(artifact.ID, models.LinkCandidate, 9999); err == nil {
		t.Error("linked to a candidate that does not exist")
	}
	if err := s.Link(artifact.ID, models.LinkTarget("planet"), 1); err == nil {
		t.Error("linked to an unknown target type")
	}
	if err := s.Link(9999, models.LinkInitiative, initiativeID); err == nil {
		t.Error("linked an artifact that does not exist")
	}

	// Detaching one link leaves the bytes and the other link alone.
	if err := s.Detach(artifact.ID, models.LinkInitiative, initiativeID); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	remaining, err := s.ListForTarget(models.LinkCandidate, candidate.ID)
	if err != nil || len(remaining) != 1 {
		t.Fatalf("the other link did not survive detaching: %v, %+v", err, remaining)
	}
	stored, err := s.bytes(artifact.ID)
	if err != nil || !bytes.Equal(stored, data) {
		t.Errorf("detaching changed the bytes: %v", err)
	}

	// Detaching something that is not attached is an error.
	if err := s.Detach(artifact.ID, models.LinkInitiative, initiativeID); err == nil {
		t.Error("detaching a link that is not there succeeded")
	}
}

func TestOrphanLibrary(t *testing.T) {
	gdb := newTestDB(t)
	s := NewArtifactService(gdb)
	initiativeID := anInitiative(t, gdb, "Records")
	data := []byte("evidence that outlives its links")

	// Ingested with no target: an orphan from the start.
	unattached, err := s.create("Pasted note", "", "pasted", data, "", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	orphans, err := s.ListOrphans()
	if err != nil {
		t.Fatalf("ListOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].ID != unattached.ID {
		t.Fatalf("orphan listing is %+v", orphans)
	}

	// Linked artifacts are not orphans.
	attached, err := s.create("", "resume.pdf", "", data, models.LinkInitiative, initiativeID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	orphans, err = s.ListOrphans()
	if err != nil {
		t.Fatalf("ListOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].ID != unattached.ID {
		t.Errorf("a linked artifact appeared in the orphan library: %+v", orphans)
	}

	// Detaching the last link makes it an orphan, with everything intact.
	if err := s.Detach(attached.ID, models.LinkInitiative, initiativeID); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	orphans, err = s.ListOrphans()
	if err != nil {
		t.Fatalf("ListOrphans: %v", err)
	}
	if len(orphans) != 2 {
		t.Fatalf("orphan library holds %d, want both", len(orphans))
	}
	got, err := s.Get(attached.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.OriginalFilename != "resume.pdf" || got.SHA256 != attached.SHA256 ||
		got.MediaType != attached.MediaType || !got.CapturedAt.Equal(attached.CapturedAt) {
		t.Errorf("orphan lost provenance: %+v", got)
	}
	stored, err := s.bytes(attached.ID)
	if err != nil || !bytes.Equal(stored, data) {
		t.Errorf("orphan lost its bytes: %v", err)
	}

	// Re-linking takes it back out of the library.
	if err := s.Link(attached.ID, models.LinkInitiative, initiativeID); err != nil {
		t.Fatalf("Link: %v", err)
	}
	orphans, err = s.ListOrphans()
	if err != nil {
		t.Fatalf("ListOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].ID != unattached.ID {
		t.Errorf("re-linked artifact is still listed as an orphan: %+v", orphans)
	}
}

func TestFailedFirstLinkLeavesNothingBehind(t *testing.T) {
	s := newArtifactService(t)

	if _, err := s.create("", "resume.pdf", "", []byte("x"), models.LinkInitiative, 9999); err == nil {
		t.Fatal("create succeeded with a target that does not exist")
	}

	all, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("a half-created artifact survived: %+v", all)
	}
	orphans, err := s.ListOrphans()
	if err != nil {
		t.Fatalf("ListOrphans: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("the rollback left an orphan: %+v", orphans)
	}
	links, err := s.Links(1)
	if err != nil {
		t.Fatalf("Links: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("the rollback left a dangling link: %+v", links)
	}
}

func TestArtifactsAreSharedNotOwnedByAnInitiative(t *testing.T) {
	gdb := newTestDB(t)
	s := NewArtifactService(gdb)
	initiatives := NewInitiativeService(gdb)
	initiativeID := anInitiative(t, gdb, "Doomed")
	records := NewRecordService(gdb)
	candidate, err := records.CreateCandidate(models.Candidate{FullName: "Priya Raman"})
	if err != nil {
		t.Fatalf("CreateCandidate: %v", err)
	}

	artifact, err := s.create("", "resume.pdf", "", []byte("x"), models.LinkInitiative, initiativeID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Link(artifact.ID, models.LinkCandidate, candidate.ID); err != nil {
		t.Fatalf("Link: %v", err)
	}

	if err := initiatives.Delete(initiativeID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(artifact.ID); err != nil {
		t.Fatalf("deleting an initiative removed a recruiter-added artifact: %v", err)
	}
	linked, err := s.ListForTarget(models.LinkCandidate, candidate.ID)
	if err != nil || len(linked) != 1 {
		t.Errorf("the candidate's link did not survive: %v, %+v", err, linked)
	}
}

func TestDuplicateBytesOnTheSameTargetAreRefused(t *testing.T) {
	gdb := newTestDB(t)
	s := NewArtifactService(gdb)
	initiative := anInitiative(t, gdb, "Dedup")
	data := []byte("the very same bytes")

	if _, err := s.create("Resume 4.0.pdf", "resume.pdf", "", data, models.LinkInitiative, initiative); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := s.create("Resume 4.0 again.pdf", "resume.pdf", "", data, models.LinkInitiative, initiative)
	if err == nil {
		t.Fatal("identical bytes were attached to the same target twice")
	}
	// The refusal names the artifact already there, so the user can find it.
	if !strings.Contains(err.Error(), "Resume 4.0.pdf") || !strings.Contains(err.Error(), "already attached") {
		t.Errorf("refusal does not name the existing artifact: %v", err)
	}

	// An orphan ingestion is checked against other orphans the same way.
	if _, err := s.create("Loose copy", "resume.pdf", "", data, "", 0); err != nil {
		t.Fatalf("orphan create: %v", err)
	}
	if _, err := s.create("Loose copy 2", "resume.pdf", "", data, "", 0); err == nil {
		t.Fatal("identical orphan bytes were ingested twice")
	}
}
