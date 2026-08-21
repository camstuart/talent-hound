package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/cloud"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/scrub"
)

// CloudService is the one deliberate exception to running locally.
//
// The whole design problem is making the exception incapable of becoming the
// rule. Three things do that: a deny list that is code rather than
// configuration, consent that cannot generalize across initiative, endpoint, or
// task, and a preview that is the payload rather than a description of it.
type CloudService struct {
	db       *gorm.DB
	model    Classifier
	records  *RecordService
	profiles *CandidateProfileService
	// key is read at call time from the credential store; nothing here stores it.
	credentials *CredentialService
}

// NewCloudService wires the cloud override to the evidence it may draw on.
func NewCloudService(
	db *gorm.DB, model Classifier, records *RecordService,
	profiles *CandidateProfileService, credentials *CredentialService,
) *CloudService {
	return &CloudService{db: db, model: model, records: records,
		profiles: profiles, credentials: credentials}
}

// addressable is a transport that can say where it sends. A transport that
// cannot is a test double, and a test double is where it says it is.
type addressable interface {
	Endpoint() string
}

// cloudTimeout bounds one cloud request.
const cloudTimeout = 2 * time.Minute

// Endpoint is the configured cloud endpoint and its revision.
type CloudEndpoint struct {
	URL   string `json:"url"`
	Model string `json:"model"`
	// Revision changes whenever the configuration does, which is what makes
	// every prior approval stop matching.
	Revision int `json:"revision"`
}

// Configure sets the cloud endpoint, as a new revision.
//
// Nothing sweeps the old approvals: they are approvals for a configuration that
// no longer exists, and they simply stop matching.
func (s *CloudService) Configure(rawURL, model string) (*CloudEndpoint, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("a cloud endpoint needs a URL")
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("the cloud endpoint must be an absolute http or https URL, got %q", rawURL)
	}

	current, err := s.Endpoint()
	if err != nil {
		return nil, err
	}
	next := models.CloudEndpointRow{URL: rawURL, Model: strings.TrimSpace(model), Revision: 1}
	if current != nil {
		if current.URL == rawURL && current.Model == strings.TrimSpace(model) {
			// Reconfiguring identically is not a change, so approvals survive —
			// the same rule the model registry uses for its assignments.
			return current, nil
		}
		next.Revision = current.Revision + 1
	}
	if err := s.db.Create(&next).Error; err != nil {
		return nil, fmt.Errorf("configuring the cloud endpoint: %w", err)
	}
	return &CloudEndpoint{URL: next.URL, Model: next.Model, Revision: next.Revision}, nil
}

// Endpoint returns the current cloud endpoint, or nil when none is configured.
func (s *CloudService) Endpoint() (*CloudEndpoint, error) {
	var row models.CloudEndpointRow
	err := s.db.Order("revision desc").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the cloud endpoint: %w", err)
	}
	return &CloudEndpoint{URL: row.URL, Model: row.Model, Revision: row.Revision}, nil
}

// Remove clears the cloud endpoint entirely.
func (s *CloudService) Remove() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&models.CloudEndpointRow{}).Error; err != nil {
			return fmt.Errorf("removing the cloud endpoint: %w", err)
		}
		// And everything approved for it. An approval is approval to send this
		// task to that endpoint, and it cannot outlive the endpoint it names.
		//
		// It used to. Revisions restart at one when no endpoint exists, so
		// revoking one provider and configuring a different one gave the new
		// one revision one again — and every consent keyed to revision one,
		// granted for a provider that is gone, matched it. A task the recruiter
		// approved for one company was approved for another without anyone
		// being asked.
		//
		// Deleting the consents here is what makes restarting at one safe.
		if err := tx.Where("1 = 1").Delete(&models.CloudConsent{}).Error; err != nil {
			return fmt.Errorf("revoking what was approved for it: %w", err)
		}
		return nil
	})
}

// TaskState is what a screen needs to say about one task.
type TaskState struct {
	Task string `json:"task"`
	// Denied means permanently refused: no approval can enable it.
	Denied bool `json:"denied"`
	// Reason explains a denial, or why an eligible task is not yet usable.
	Reason string `json:"reason"`
	// Approved means approved for the current endpoint revision and this
	// initiative.
	Approved bool `json:"approved"`
}

// Tasks reports every task's state, including the ones that can never be
// enabled — a screen that only lists what is off invites someone to turn on
// what is forbidden.
func (s *CloudService) Tasks(initiativeID uint) ([]TaskState, error) {
	endpoint, err := s.Endpoint()
	if err != nil {
		return nil, err
	}
	out := []TaskState{}
	for _, task := range cloud.Eligible {
		state := TaskState{Task: string(task)}
		if endpoint == nil {
			state.Reason = "no cloud endpoint is configured"
			out = append(out, state)
			continue
		}
		approved, err := s.approved(initiativeID, endpoint.Revision, task)
		if err != nil {
			return nil, err
		}
		state.Approved = approved
		if !approved {
			state.Reason = "not approved for this initiative and endpoint"
		}
		out = append(out, state)
	}
	for _, task := range cloud.Denied() {
		out = append(out, TaskState{
			Task:   string(task),
			Denied: true,
			Reason: cloud.Allowed(task).Error(),
		})
	}
	return out, nil
}

// Approve records consent for one initiative, one endpoint revision, and one
// task.
func (s *CloudService) Approve(initiativeID uint, task string) error {
	if err := cloud.Allowed(cloud.Task(task)); err != nil {
		return err
	}
	endpoint, err := s.Endpoint()
	if err != nil {
		return err
	}
	if endpoint == nil {
		return fmt.Errorf("configure a cloud endpoint before approving anything for it")
	}
	row := models.CloudConsent{
		InitiativeID:     initiativeID,
		EndpointRevision: endpoint.Revision,
		Task:             task,
		ApprovedAt:       time.Now().UTC(),
	}
	err = s.db.Where("initiative_id = ? AND endpoint_revision = ? AND task = ?",
		initiativeID, endpoint.Revision, task).
		Assign(map[string]any{"approved_at": row.ApprovedAt, "revoked_at": nil}).
		FirstOrCreate(&row).Error
	if err != nil {
		return fmt.Errorf("recording the approval: %w", err)
	}
	return nil
}

// Revoke takes an approval back. It takes effect before the next request
// because the next request looks it up.
func (s *CloudService) Revoke(initiativeID uint, task string) error {
	endpoint, err := s.Endpoint()
	if err != nil {
		return err
	}
	if endpoint == nil {
		return nil
	}
	now := time.Now().UTC()
	err = s.db.Model(&models.CloudConsent{}).
		Where("initiative_id = ? AND endpoint_revision = ? AND task = ?",
			initiativeID, endpoint.Revision, task).
		Update("revoked_at", now).Error
	if err != nil {
		return fmt.Errorf("revoking the approval: %w", err)
	}
	return nil
}

// approved reports whether this exact combination is approved.
//
// All three or nothing: there is no fallback to a broader approval, because the
// fallback is the generalization the boundary exists to prevent.
func (s *CloudService) approved(initiativeID uint, revision int, task cloud.Task) (bool, error) {
	var row models.CloudConsent
	err := s.db.Where("initiative_id = ? AND endpoint_revision = ? AND task = ? AND revoked_at IS NULL",
		initiativeID, revision, string(task)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking the approval: %w", err)
	}
	return true, nil
}

// Payload is what would be sent, exactly.
type Payload struct {
	Task string `json:"task"`
	// Text is the request body's prompt, with identifiers already replaced —
	// substitution happens before the preview, so the recruiter previews what
	// will actually be sent.
	Text string `json:"text"`
	// Endpoint and Model say where it would go.
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

// PreviewInput is a request to see a payload.
type PreviewInput struct {
	InitiativeID uint   `json:"initiativeId"`
	CandidateID  uint   `json:"candidateId"`
	Task         string `json:"task"`
	// Text is what the caller wants to send — a question, a requirement, a
	// draft brief. For chat it is chosen explicitly for each send.
	Text string `json:"text"`
}

// Preview builds the payload and writes nothing.
func (s *CloudService) Preview(in PreviewInput) (*Payload, error) {
	if err := cloud.Allowed(cloud.Task(in.Task)); err != nil {
		return nil, err
	}
	endpoint, err := s.Endpoint()
	if err != nil {
		return nil, err
	}
	if endpoint == nil {
		return nil, fmt.Errorf("no cloud endpoint is configured")
	}

	ids := scrub.Identifiers{}
	if in.CandidateID != 0 {
		c, err := s.records.GetCandidate(in.CandidateID)
		if err != nil {
			return nil, err
		}
		names := []string{c.FullName}
		if c.PreferredName != "" {
			names = append(names, c.PreferredName)
		}
		ids = scrub.Identifiers{
			Names: names, Emails: c.Emails, Phones: c.Phones, Address: c.Location,
		}
	}
	return &Payload{
		Task:     in.Task,
		Text:     cloud.Redact(in.Text, ids),
		Endpoint: endpoint.URL,
		Model:    endpoint.Model,
	}, nil
}

// cloudCategories names what kinds of thing this payload disclosed.
//
// The base is what a payload for these tasks carries. An organization name is
// added when one is there, because Redact removes direct identifiers and leaves
// organizations standing — the same choice the search side makes, for the same
// reason: naming a company is ordinary recruiting and naming a person is not.
//
// A direct identifier cannot be here, because Send refuses a payload still
// carrying one. It is named anyway if it somehow is, because a disclosure record
// that cannot describe the unexpected case is a record of the expected one.
func cloudCategories(text string) string {
	kinds := []string{"approved profile aspects and selected evidence snippets"}
	organization, identifier := false, false
	for _, found := range scrub.Detect(text, scrub.Identifiers{}) {
		if found.Kind == scrub.KindOrganization {
			organization = true
			continue
		}
		identifier = true
	}
	if organization {
		kinds = append(kinds, "an organization name")
	}
	if identifier {
		kinds = append(kinds, "a direct identifier")
	}
	return strings.Join(kinds, ", ")
}

// Send transmits a previewed payload, unchanged.
//
// Everything it refuses, it refuses before transmitting: the boundary, the
// approval, and the credential. A refusal that reached the endpoint first would
// be a disclosure that the refusal then pretended had not happened.
func (s *CloudService) Send(initiativeID uint, payload Payload) (string, error) {
	if err := cloud.Allowed(cloud.Task(payload.Task)); err != nil {
		return "", err
	}
	// The payload arrives from the caller, and redaction happens in Preview. A
	// caller that skipped it — a bound method is reachable from anything the
	// window runs — would transmit whatever it was handed, and the recruiter
	// would have approved a preview of something else.
	//
	// Checked rather than redone. Redacting here would send text that differs
	// from what was previewed, which is the opposite failure: approval has to
	// be about the thing that leaves. Redaction is idempotent, so a previewed
	// payload passes through this unchanged and raw text does not.
	if cloud.Redact(payload.Text, scrub.Identifiers{}) != payload.Text {
		return "", fmt.Errorf("this payload still carries a direct identifier: preview it first")
	}
	endpoint, err := s.Endpoint()
	if err != nil {
		return "", err
	}
	if endpoint == nil {
		return "", fmt.Errorf("no cloud endpoint is configured")
	}
	if payload.Endpoint != endpoint.URL {
		// The payload was previewed against a configuration that has since
		// changed, which is exactly what an endpoint change is supposed to stop.
		return "", fmt.Errorf("the cloud endpoint changed since this payload was previewed — preview it again")
	}
	approved, err := s.approved(initiativeID, endpoint.Revision, cloud.Task(payload.Task))
	if err != nil {
		return "", err
	}
	if !approved {
		return "", fmt.Errorf("%s is not approved for this initiative and endpoint", payload.Task)
	}
	has, err := s.credentials.Has("cloud")
	if err != nil || !has {
		return "", fmt.Errorf("no cloud credential is stored — the provider is disabled")
	}

	// And the transport actually goes where the recruiter approved.
	//
	// It does not. This build wires one client, pointed at the local runtime,
	// and hands the same instance to this service — so a payload previewed for
	// an endpoint, approved for that endpoint and recorded as disclosed to it
	// was answered by the model on this machine. Safe, and a lie in both
	// directions: the recruiter was told their text went somewhere it did not,
	// and the disclosure record said a disclosure happened that had not.
	//
	// Refused rather than quietly answered. Wiring a real OpenAI-compatible
	// transport is the fix, and it is the one change in this product that makes
	// candidate-derived text leave the machine — so it belongs to somebody who
	// can watch it happen, not to a guess made where it cannot be observed.
	if at, ok := s.model.(addressable); ok && at.Endpoint() != endpoint.URL {
		return "", fmt.Errorf(
			"this build has no cloud transport: it would send to %s rather than to %s, "+
				"so nothing was sent", at.Endpoint(), endpoint.URL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cloudTimeout)
	defer cancel()
	answer, sendErr := s.model.Chat(ctx, endpoint.Model, payload.Text, nil)

	// The request was transmitted, so the disclosure happened — whether or not
	// the provider then answered.
	id := initiativeID
	event := models.DisclosureEvent{
		OccurredAt:   time.Now().UTC(),
		Provider:     "cloud",
		Task:         payload.Task,
		Categories:   cloudCategories(payload.Text),
		InitiativeID: &id,
	}
	if err := s.db.Create(&event).Error; err != nil {
		return "", fmt.Errorf("recording the disclosure: %w", err)
	}
	if sendErr != nil {
		return "", fmt.Errorf("the cloud provider did not answer")
	}
	return answer, nil
}
