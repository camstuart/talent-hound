package main

import (
	"fmt"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/cloud"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// cloudEnv is a qaEnv with the cloud override wired in.
type cloudEnv struct {
	*qaEnv
	cloud       *CloudService
	credentials *CredentialService
	store       *memoryStore
}

func newCloudEnv(t *testing.T) *cloudEnv {
	t.Helper()
	base := newQAEnv(t)
	store := newMemoryStore()
	credentials := &CredentialService{store: store}
	return &cloudEnv{
		qaEnv:       base,
		cloud:       NewCloudService(base.db, base.model, base.records, base.profiles, credentials),
		credentials: credentials,
		store:       store,
	}
}

func (e *cloudEnv) configure(t *testing.T, rawURL string) *CloudEndpoint {
	t.Helper()
	endpoint, err := e.cloud.Configure(rawURL, "cloud-model")
	if err != nil {
		t.Fatalf("configuring: %v", err)
	}
	return endpoint
}

// withKey stores a cloud credential, which sending requires.
func (e *cloudEnv) withKey(t *testing.T) {
	t.Helper()
	if err := e.credentials.Store("cloud", "not-a-real-key-CLOUD-7f31a2"); err != nil {
		t.Fatalf("storing the key: %v", err)
	}
}

// A screen that only lists what is off invites someone to turn on what is
// forbidden.
func TestEveryTaskReportsWhetherItCanEverBeEnabled(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")

	states, err := e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	byTask := map[string]TaskState{}
	for _, s := range states {
		byTask[s.Task] = s
	}
	for _, task := range []cloud.Task{cloud.RoleExtraction, cloud.Assessment, cloud.Drafting, cloud.Chat} {
		state, ok := byTask[string(task)]
		if !ok {
			t.Fatalf("%s is missing from the task list", task)
		}
		if state.Denied {
			t.Errorf("%s is reported as permanently denied", task)
		}
		if state.Approved {
			t.Errorf("%s is approved before anyone approved it", task)
		}
	}
	for _, task := range []cloud.Task{cloud.CandidateExtraction, cloud.Embedding, cloud.RawArtifact} {
		state, ok := byTask[string(task)]
		if !ok {
			t.Fatalf("%s is missing from the task list — a denial nobody can see is not a control", task)
		}
		if !state.Denied {
			t.Errorf("%s is not reported as denied", task)
		}
		if !strings.Contains(state.Reason, "local-only") {
			t.Errorf("%s does not say why: %q", task, state.Reason)
		}
	}
}

func TestADeniedTaskCannotBeApprovedOrPreviewedOrSent(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)

	for _, task := range cloud.Denied() {
		t.Run(string(task), func(t *testing.T) {
			if err := e.cloud.Approve(e.initiative, string(task)); err == nil {
				t.Fatalf("%s was approved", task)
			}
			if _, err := e.cloud.Preview(PreviewInput{
				InitiativeID: e.initiative, Task: string(task), Text: "anything",
			}); err == nil {
				t.Fatalf("%s was previewed", task)
			}
			if _, err := e.cloud.Send(e.initiative, Payload{
				Task: string(task), Text: "anything",
				Endpoint: "https://api.example-cloud.invalid/v1",
			}); err == nil {
				t.Fatalf("%s was sent", task)
			}
		})
	}
	// And nothing reached the endpoint, under any of it.
	if e.model.callCount() != 0 {
		t.Fatalf("a denied task reached the model %d times", e.model.callCount())
	}
}

func TestConsentDoesNotGeneralize(t *testing.T) {
	e := newCloudEnv(t)
	endpoint := e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("approving: %v", err)
	}
	payload := Payload{Task: string(cloud.Drafting), Text: "a pitch", Endpoint: endpoint.URL}
	e.model.respond = func(string) string { return "ok" }

	// The approved combination works, and returns what the provider said.
	answer, err := e.cloud.Send(e.initiative, payload)
	if err != nil {
		t.Fatalf("the approved task was refused: %v", err)
	}
	if answer != "ok" {
		t.Fatalf("the provider's answer was %q, want what it actually said", answer)
	}

	// Another task in the same initiative is not authorized.
	other := payload
	other.Task = string(cloud.Assessment)
	if _, err := e.cloud.Send(e.initiative, other); err == nil {
		t.Fatal("approving drafting authorized assessment")
	}

	// Another initiative is not authorized.
	inits := NewInitiativeService(e.db)
	second, err := inits.Create("Other "+t.Name(), models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating the other initiative: %v", err)
	}
	if _, err := e.cloud.Send(second.ID, payload); err == nil {
		t.Fatal("approving one initiative authorized another")
	}
}

func TestChangingTheEndpointResetsEveryApproval(t *testing.T) {
	e := newCloudEnv(t)
	first := e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("approving: %v", err)
	}
	e.model.respond = func(string) string { return "ok" }
	if _, err := e.cloud.Send(e.initiative, Payload{
		Task: string(cloud.Drafting), Text: "a pitch", Endpoint: first.URL,
	}); err != nil {
		t.Fatalf("the approved task was refused: %v", err)
	}

	// The endpoint changes.
	second := e.configure(t, "https://api.another-cloud.invalid/v1")
	if second.Revision == first.Revision {
		t.Fatal("changing the endpoint did not make a new revision")
	}

	// The old approval no longer matches, and the refusal happens before
	// anything is transmitted.
	before := e.model.callCount()
	if _, err := e.cloud.Send(e.initiative, Payload{
		Task: string(cloud.Drafting), Text: "a pitch", Endpoint: second.URL,
	}); err == nil {
		t.Fatal("an approval survived an endpoint change")
	}
	if e.model.callCount() != before {
		t.Fatal("a refused request still reached the provider")
	}

	// Every task shows as unapproved again.
	states, err := e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	for _, s := range states {
		if s.Approved {
			t.Errorf("%s survived the endpoint change as approved", s.Task)
		}
	}
}

// Reconfiguring to the same thing is not a change.
func TestReconfiguringIdenticallyKeepsApprovals(t *testing.T) {
	e := newCloudEnv(t)
	first := e.configure(t, "https://api.example-cloud.invalid/v1")
	if err := e.cloud.Approve(e.initiative, string(cloud.Chat)); err != nil {
		t.Fatalf("approving: %v", err)
	}
	again := e.configure(t, "https://api.example-cloud.invalid/v1")
	if again.Revision != first.Revision {
		t.Fatalf("an identical configuration made revision %d from %d", again.Revision, first.Revision)
	}
	states, err := e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	for _, s := range states {
		if s.Task == string(cloud.Chat) && !s.Approved {
			t.Fatal("an identical reconfiguration dropped an approval")
		}
	}
}

func TestRevocationTakesEffectBeforeTheNextRequest(t *testing.T) {
	e := newCloudEnv(t)
	endpoint := e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	for _, task := range []cloud.Task{cloud.Drafting, cloud.Chat} {
		if err := e.cloud.Approve(e.initiative, string(task)); err != nil {
			t.Fatalf("approving %s: %v", task, err)
		}
	}
	e.model.respond = func(string) string { return "ok" }

	if err := e.cloud.Revoke(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("revoking: %v", err)
	}
	before := e.model.callCount()
	if _, err := e.cloud.Send(e.initiative, Payload{
		Task: string(cloud.Drafting), Text: "a pitch", Endpoint: endpoint.URL,
	}); err == nil {
		t.Fatal("a revoked task was sent")
	}
	if e.model.callCount() != before {
		t.Fatal("a revoked request still reached the provider")
	}

	// Revoking one does not touch another.
	if _, err := e.cloud.Send(e.initiative, Payload{
		Task: string(cloud.Chat), Text: "a question", Endpoint: endpoint.URL,
	}); err != nil {
		t.Fatalf("revoking drafting also revoked chat: %v", err)
	}
}

func TestRemovingTheCredentialDisablesTheProviderWithoutDeletingAnything(t *testing.T) {
	e := newCloudEnv(t)
	endpoint := e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("approving: %v", err)
	}
	e.model.respond = func(string) string { return "ok" }
	payload := Payload{Task: string(cloud.Drafting), Text: "a pitch", Endpoint: endpoint.URL}
	if _, err := e.cloud.Send(e.initiative, payload); err != nil {
		t.Fatalf("sending: %v", err)
	}

	if err := e.credentials.Delete("cloud"); err != nil {
		t.Fatalf("removing the key: %v", err)
	}
	before := e.model.callCount()
	if _, err := e.cloud.Send(e.initiative, payload); err == nil {
		t.Fatal("a request was sent with no credential")
	}
	if e.model.callCount() != before {
		t.Fatal("a credential-less request still reached the provider")
	}

	// The approval and the endpoint survive: removing a key disables a
	// provider, it does not delete local information.
	states, err := e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	approved := false
	for _, s := range states {
		if s.Task == string(cloud.Drafting) && s.Approved {
			approved = true
		}
	}
	if !approved {
		t.Error("removing the credential dropped the approval")
	}
}

// Substitution happens before the preview, so the recruiter previews what will
// actually be sent.
func TestIdentifiersAreReplacedBeforeThePreview(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")
	c, err := e.records.CreateCandidate(models.Candidate{
		FullName: "Kalinda Reyes",
		Emails:   models.StringList{"kalinda.reyes@example.invalid"},
		Phones:   models.StringList{"+61 400 123 456"},
		Location: "12 Wattle Street, Fitzroy",
	})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}

	payload, err := e.cloud.Preview(PreviewInput{
		InitiativeID: e.initiative, CandidateID: c.ID, Task: string(cloud.Drafting),
		Text: "Kalinda Reyes (kalinda.reyes@example.invalid, +61 400 123 456, " +
			"12 Wattle Street, Fitzroy) has five years of production Go.",
	})
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	for _, identifier := range []string{
		"Kalinda", "Reyes", "kalinda.reyes@example.invalid", "400 123 456", "Wattle Street",
	} {
		if strings.Contains(payload.Text, identifier) {
			t.Errorf("%q survived into the previewed payload: %q", identifier, payload.Text)
		}
	}
	if !strings.Contains(payload.Text, "five years of production Go") {
		t.Errorf("redaction removed the professional content: %q", payload.Text)
	}
	if !strings.Contains(payload.Text, cloud.NamePlaceholder) {
		t.Errorf("the payload does not read as being about someone: %q", payload.Text)
	}
}

// A preview built by one path and a request by another diverge the day someone
// adds a header.
func TestThePreviewedPayloadIsTheSentPayload(t *testing.T) {
	e := newCloudEnv(t)
	endpoint := e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("approving: %v", err)
	}

	var sent string
	e.model.respond = func(prompt string) string {
		sent = prompt
		return "ok"
	}
	payload, err := e.cloud.Preview(PreviewInput{
		InitiativeID: e.initiative, Task: string(cloud.Drafting),
		Text: "Write a pitch about five years of production Go.",
	})
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if payload.Endpoint != endpoint.URL {
		t.Fatalf("the preview names %q, the endpoint is %q", payload.Endpoint, endpoint.URL)
	}
	if _, err := e.cloud.Send(e.initiative, *payload); err != nil {
		t.Fatalf("sending: %v", err)
	}
	if sent != payload.Text {
		t.Fatalf("the provider received %q, the recruiter previewed %q", sent, payload.Text)
	}
}

func TestPreviewingSendsNothingAndRecordsNothing(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")

	if _, err := e.cloud.Preview(PreviewInput{
		InitiativeID: e.initiative, Task: string(cloud.Chat), Text: "a question",
	}); err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if e.model.callCount() != 0 {
		t.Fatal("previewing reached the provider")
	}
	var events int64
	if err := e.db.Model(&models.DisclosureEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if events != 0 {
		t.Fatalf("previewing recorded %d disclosure events", events)
	}
}

func TestEverySentCloudRequestIsAuditedWithNoPayload(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Assessment)); err != nil {
		t.Fatalf("approving: %v", err)
	}
	e.model.respond = func(string) string { return "ok" }

	const distinctive = "Quokkabeam telemetry rollout in Fremantle"
	payload, err := e.cloud.Preview(PreviewInput{
		InitiativeID: e.initiative, Task: string(cloud.Assessment), Text: distinctive,
	})
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if _, err := e.cloud.Send(e.initiative, *payload); err != nil {
		t.Fatalf("sending: %v", err)
	}

	events := []models.DisclosureEvent{}
	if err := e.db.Where("provider = ?", "cloud").Find(&events).Error; err != nil {
		t.Fatalf("listing events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("one request produced %d events", len(events))
	}
	event := events[0]
	if event.Task != string(cloud.Assessment) {
		t.Errorf("the event names task %q", event.Task)
	}
	if event.InitiativeID == nil || *event.InitiativeID != e.initiative {
		t.Errorf("the event does not name the initiative: %+v", event)
	}
	blob := fmt.Sprintf("%+v", event)
	for _, content := range []string{"Quokkabeam", "telemetry", "Fremantle"} {
		if strings.Contains(blob, content) {
			t.Fatalf("the disclosure event contains %q: %s", content, blob)
		}
	}
}

func TestALocalRequestCreatesNoDisclosureEvent(t *testing.T) {
	e := newCloudEnv(t)
	// A local classification, which is what every earlier phase does.
	e.assignClassify(t, "synthetic-classify")
	id := e.draftableCandidate(t)
	_ = id

	var events int64
	if err := e.db.Model(&models.DisclosureEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if events != 0 {
		t.Fatalf("local work recorded %d disclosure events", events)
	}
}

func TestEverythingStartsDenied(t *testing.T) {
	e := newCloudEnv(t)
	// No endpoint at all.
	states, err := e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	for _, s := range states {
		if s.Approved {
			t.Errorf("%s is approved with no endpoint configured", s.Task)
		}
	}
	// And approving before configuring is refused.
	if err := e.cloud.Approve(e.initiative, string(cloud.Chat)); err == nil {
		t.Fatal("a task was approved with no endpoint")
	}
}

func TestAnUnapprovedTaskTransmitsNothing(t *testing.T) {
	e := newCloudEnv(t)
	endpoint := e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	e.model.respond = func(string) string { return "ok" }

	if _, err := e.cloud.Send(e.initiative, Payload{
		Task: string(cloud.Chat), Text: "a question", Endpoint: endpoint.URL,
	}); err == nil {
		t.Fatal("an unapproved task was sent")
	}
	if e.model.callCount() != 0 {
		t.Fatal("an unapproved request reached the provider")
	}
	var events int64
	if err := e.db.Model(&models.DisclosureEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if events != 0 {
		t.Fatalf("an unapproved request recorded %d events", events)
	}
}

// The local roles stay required: the cloud is an override, never a replacement.
func TestTheCloudEndpointDoesNotReplaceTheLocalConfiguration(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")

	// The embed role still refuses a non-local endpoint, whatever the cloud
	// configuration says.
	if _, err := e.registry.Assign(AssignInput{
		Role: models.RoleEmbed, Endpoint: "https://api.example-cloud.invalid/v1", Model: "cloud-embed",
	}); err == nil {
		t.Fatal("configuring a cloud endpoint let the embed role point at it")
	}
	// And local work still runs.
	e.assignClassify(t, "synthetic-classify")
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Kalinda Reyes", Location: "Melbourne"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	if _, err := e.profiles.AddAspect(c.ID, profile.Aspect{
		Type: profile.Skill, Wording: "Five years of production Go",
	}); err != nil {
		t.Fatalf("local work failed with a cloud endpoint configured: %v", err)
	}
}

// Revocation, which FR-11 requires alongside visible configuration and which
// nothing exercised.
//
// Removing the endpoint has to take the approvals with it. An approval is
// approval to send this task to that endpoint, and leaving one behind after the
// endpoint is gone means the next endpoint configured inherits a permission
// nobody granted it.
func TestRemovingTheEndpointRevokesWhatWasApprovedForIt(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)

	approvable := ""
	states, err := e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("reading tasks: %v", err)
	}
	for _, s := range states {
		if !s.Denied {
			approvable = s.Task
		}
	}
	if approvable == "" {
		t.Fatal("no task can be approved, so this proves nothing")
	}
	if err := e.cloud.Approve(e.initiative, approvable); err != nil {
		t.Fatalf("approving %s: %v", approvable, err)
	}

	if err := e.cloud.Remove(); err != nil {
		t.Fatalf("removing the endpoint: %v", err)
	}
	endpoint, err := e.cloud.Endpoint()
	if err != nil {
		t.Fatalf("reading the endpoint: %v", err)
	}
	if endpoint != nil {
		t.Fatal("the endpoint survived its own removal")
	}

	// Configure a different endpoint. It must start with nothing approved.
	e.configure(t, "https://api.other-cloud.invalid/v1")
	states, err = e.cloud.Tasks(e.initiative)
	if err != nil {
		t.Fatalf("reading tasks after reconfiguring: %v", err)
	}
	for _, s := range states {
		if s.Approved {
			t.Fatalf("%s is approved for an endpoint nobody approved it for", s.Task)
		}
	}
}

// Removing an endpoint that was never configured is not an error: a recruiter
// clicking revoke twice has revoked it.
func TestRemovingNothingIsNotAFailure(t *testing.T) {
	e := newCloudEnv(t)
	if err := e.cloud.Remove(); err != nil {
		t.Fatalf("removing nothing: %v", err)
	}
}

// A payload that skipped the preview is refused, and a previewed one is not
// touched.
//
// Redaction happens in Preview, and Send transmits what it is handed. That is
// deliberate — redacting again in Send could transmit text differing from what
// the recruiter approved, and approval has to be about the thing that leaves.
// It also meant a caller could hand Send raw text and have it sent, and Send is
// a bound method: anything running in the window can reach it.
//
// Redaction is idempotent, so asking whether the text is already redacted
// changes nothing about a previewed payload and refuses one that never was.
func TestAPayloadThatSkippedThePreviewIsRefused(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("approving: %v", err)
	}

	reached := false
	e.model.respond = func(string) string {
		reached = true
		return "ok"
	}

	// A real preview, so the endpoint and the task are exactly right, and then
	// the text swapped for what never went through redaction. Anything less
	// than this is refused by a different check — the first version of this
	// test built the payload by hand, and Send turned it away because the
	// endpoint field was empty, which proved nothing about identifiers.
	const line = "Kalinda Reyes, kalinda.reyes@example.invalid, +61 400 123 456 — five years of Go."
	previewed, err := e.cloud.Preview(PreviewInput{
		InitiativeID: e.initiative, Task: string(cloud.Drafting), Text: line,
	})
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	raw := *previewed
	raw.Text = line

	_, err = e.cloud.Send(e.initiative, raw)
	if err == nil {
		t.Fatal("a payload carrying an email and a phone number was sent")
	}
	if reached {
		t.Fatal("the endpoint received it before the refusal")
	}
	// The refusal says what to do, and does not quote what it refused.
	if strings.Contains(err.Error(), "kalinda") || strings.Contains(err.Error(), "400 123") {
		t.Fatalf("the refusal quoted the payload: %v", err)
	}

	// And the previewed path is unaffected: same text in, same text out.
	payload := previewed
	var sent string
	e.model.respond = func(prompt string) string {
		sent = prompt
		return "ok"
	}
	if _, err := e.cloud.Send(e.initiative, *payload); err != nil {
		t.Fatalf("sending a previewed payload: %v", err)
	}
	if sent != payload.Text {
		t.Fatalf("the provider received %q, the recruiter previewed %q", sent, payload.Text)
	}
}

// The cloud disclosure record names an organization when one was sent.
//
// Redact removes direct identifiers and leaves organizations standing — naming
// a company is ordinary recruiting and naming a person is not — so a payload
// can legitimately carry one. The record said "approved profile aspects and
// selected evidence snippets" either way, which is the same understatement the
// search side had: it is the evidence somebody checks instead of looking.
func TestTheCloudDisclosureNamesAnOrganizationThatWasSent(t *testing.T) {
	e := newCloudEnv(t)
	e.configure(t, "https://api.example-cloud.invalid/v1")
	e.withKey(t)
	if err := e.cloud.Approve(e.initiative, string(cloud.Drafting)); err != nil {
		t.Fatalf("approving: %v", err)
	}
	e.model.respond = func(string) string { return "ok" }

	latest := func(t *testing.T) string {
		t.Helper()
		rows := []models.DisclosureEvent{}
		if err := e.db.Order("id desc").Find(&rows).Error; err != nil {
			t.Fatalf("reading disclosures: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("nothing was recorded")
		}
		return rows[0].Categories
	}

	send := func(t *testing.T, text string) {
		t.Helper()
		payload, err := e.cloud.Preview(PreviewInput{
			InitiativeID: e.initiative, Task: string(cloud.Drafting), Text: text,
		})
		if err != nil {
			t.Fatalf("previewing: %v", err)
		}
		if _, err := e.cloud.Send(e.initiative, *payload); err != nil {
			t.Fatalf("sending: %v", err)
		}
	}

	t.Run("ordinary wording names only the base kinds", func(t *testing.T) {
		send(t, "Write a pitch about five years of production Go and SQLite.")
		if got := latest(t); strings.Contains(got, "organization") {
			t.Fatalf("categories = %q, which claims more than was sent", got)
		}
	})

	t.Run("a company named in the payload is named in the record", func(t *testing.T) {
		send(t, "Write a pitch about five years at Quokkastack Pty Ltd building Go services.")
		got := latest(t)
		if !strings.Contains(got, "an organization name") {
			t.Fatalf("categories = %q, and a company was in the payload", got)
		}
		if strings.Contains(got, "Quokkastack") {
			t.Fatalf("the record quotes the organization: %q", got)
		}
	})
}
