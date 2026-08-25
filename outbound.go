package main

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/scrub"
)

// outbound is the part of a search service that is about information leaving
// the machine, shared by every service that sends anything: the preview
// warnings, the credential read, and the disclosure record.
//
// It is one type rather than a copy per service because the rules here are the
// ones an audit checks, and two copies of an audit rule drift.
type outbound struct {
	db          *gorm.DB
	credentials *CredentialService
}

// describe attaches the two warnings to a query without changing it.
func (o *outbound) describe(query string, ids scrub.Identifiers) *QueryPreview {
	org, ident := scrub.Warnings(scrub.Detect(query, ids))
	return &QueryPreview{Query: query, OrganizationWarning: org, IdentifierWarning: ident}
}

// key reads a provider's credential at the moment of the request.
//
// Never at start-up: a key read then is a key read before the recruiter has
// entered one, and every later request would be refused for its absence.
func (o *outbound) key(provider string) (string, error) {
	if o.credentials == nil {
		return "", fmt.Errorf("no credential store is available for the search provider")
	}
	key, err := o.credentials.secret(provider)
	if err != nil {
		return "", fmt.Errorf("no search credential is stored — the provider is disabled")
	}
	return key, nil
}

// record writes the audit event: that information left, never what.
//
// The caller fills in provider, task, categories, and record identifiers. It
// cannot fill in a query or a result, because the row has nowhere to put one.
func (o *outbound) record(event models.DisclosureEvent) error {
	if err := o.db.Create(&event).Error; err != nil {
		return fmt.Errorf("recording the disclosure: %w", err)
	}
	return nil
}

// categories names what kinds of thing a query disclosed, starting from the
// kinds a generated query always carries and adding what detection finds in
// the query as actually sent — the recruiter may have edited it.
//
// Kinds, never the things themselves: that an identifier was sent, never which.
func (o *outbound) categories(base []string, query string, ids scrub.Identifiers) string {
	kinds := append([]string(nil), base...)
	organization, identifier := false, false
	for _, found := range scrub.Detect(query, ids) {
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
