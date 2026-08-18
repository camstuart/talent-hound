package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The application drafts; the recruiter sends. This file is why that is a
// property rather than a promise.
//
// Documentation saying "we do not send" is worth nothing the day someone adds a
// convenience. A failing test is worth something, and this one is deliberately
// crude: it does not care how a sender might have arrived.
//
// transport-check-exempt: this file names senders to assert their absence

// senders are the things an outreach transport is made of: the protocols, the
// standard-library packages, and the vendor SDKs.
var senders = []string{
	"net/smtp",
	"smtp.",
	"sendmail",
	"gomail",
	"sendgrid",
	"mailgun",
	"postmark",
	"ses.SendEmail",
	"twilio",
	"messagebird",
	"nexmo",
	"linkedin.com/api",
	"graph.microsoft.com/v1.0/me/sendMail",
	"MailComposer",
}

// exemptionMarker lets a file that names senders — in order to prove they are
// absent — say so. Anything without it is scanned.
const exemptionMarker = "transport-check-exempt: this file names senders to assert their absence"

// mustRead reads a file, returning nothing when it cannot.
func mustRead(path string) []byte {
	body, err := os.ReadFile(path) //nolint:gosec // reads this repository
	if err != nil {
		return nil
	}
	return body
}

// ports are what a mail or messaging service listens on.
var ports = []string{":25", ":465", ":587", ":2525"}

// scannedExtensions are the source files worth reading. Generated bindings and
// dependencies are excluded: this checks what this repository does.
var scannedExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".json": true,
}

// skipped directories hold dependencies, build output, and this file's own
// fixtures.
var skipped = map[string]bool{
	"node_modules": true, "bindings": true, "dist": true, "bin": true,
	".git": true, "build": true, ".e2e-db": true, "test-results": true,
	"playwright-report": true, "docs": true, "openspec": true,
}

func TestNoOutreachSenderExistsInTheRepository(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("finding the repository: %v", err)
	}

	found := []string{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// An unreadable entry is not a sender. Walking past it keeps the
			// check running over everything it can read, which is the point.
			//nolint:nilerr // an unreadable entry is not a finding
			return nil
		}
		if d.IsDir() {
			if skipped[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !scannedExtensions[filepath.Ext(path)] {
			return nil
		}
		// A file that names senders in order to assert their absence would
		// otherwise fail this check. The marker is explicit so the exemption
		// cannot be granted accidentally.
		if strings.Contains(string(mustRead(path)), exemptionMarker) {
			return nil
		}
		body, err := os.ReadFile(path) //nolint:gosec // walks this repository
		if err != nil {
			//nolint:nilerr // an unreadable file is not a finding
			return nil
		}
		text := string(body)
		lowered := strings.ToLower(text)
		for _, sender := range senders {
			if strings.Contains(lowered, strings.ToLower(sender)) {
				found = append(found, path+" contains "+sender)
			}
		}
		for _, port := range ports {
			// Ports are checked against the original text so a version string
			// like "1:25" in prose is less likely to trip it.
			if strings.Contains(text, "\""+port+"\"") {
				found = append(found, path+" dials "+port)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	if len(found) > 0 {
		t.Fatalf("the application must not be able to send outreach, but:\n  %s",
			strings.Join(found, "\n  "))
	}
}

// The service surface is the other half: nothing bound to the frontend offers
// to send, and no credential is collected for a sender.
func TestNoServiceMethodOffersToSend(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("finding the repository: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading the repository: %v", err)
	}

	// Method names that would mean transport. "Send" alone is not one of them:
	// DiscoveryService.Send transmits a search query to a search provider,
	// which is a disclosure with its own preview and audit, and is not outreach.
	forbidden := []string{
		"func (s *DraftService) Send",
		"func (s *DraftService) Email",
		"func (s *DraftService) Deliver",
		"SendEmail(", "SendMessage(", "SendSMS(", "SendOutreach(",
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			entry.Name() == "transport_test.go" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, entry.Name())) //nolint:gosec // reads this repository
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		for _, name := range forbidden {
			if strings.Contains(string(body), name) {
				t.Errorf("%s declares %s — the application cannot send outreach", entry.Name(), name)
			}
		}
	}
}

// The credential store knows two providers, and neither is a sender.
func TestNoCredentialIsCollectedForASender(t *testing.T) {
	for _, provider := range Providers {
		lowered := strings.ToLower(provider)
		for _, sender := range []string{"smtp", "mail", "sms", "twilio", "sendgrid", "linkedin"} {
			if strings.Contains(lowered, sender) {
				t.Errorf("the credential store collects a key for %q, which is a sender", provider)
			}
		}
	}
}

// The audit vocabulary has no word for a send, which is deliberate: a
// vocabulary that could express one is a vocabulary someone reads as a
// capability.
func TestTheAuditVocabularyCannotExpressASend(t *testing.T) {
	for _, task := range []string{"sent", "send", "delivered", "emailed", "messaged"} {
		if strings.Contains(strings.ToLower(taskVocabulary()), task) {
			t.Errorf("the audit vocabulary contains %q", task)
		}
	}
}

// taskVocabulary is every audit task this build can record.
func taskVocabulary() string {
	return strings.Join([]string{"role_search", "copied_out"}, " ")
}

// reporters are what telemetry is made of: the SDKs, the hosted endpoints, and
// the words a background reporter is usually called by.
//
// The PRD says no telemetry — not telemetry that is off by default, because a
// reporter with a flag is a reporter.
var reporters = []string{
	"sentry", "posthog", "mixpanel", "segment.io", "segment.com", "amplitude",
	"datadoghq", "bugsnag", "rollbar", "new relic", "newrelic", "opentelemetry",
	"go.opentelemetry.io", "google-analytics", "googletagmanager", "plausible.io",
	"analytics.", "telemetry.", "crashreport", "usage_reporting",
}

func TestNoTelemetryExistsInTheRepository(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("finding the repository: %v", err)
	}

	found := []string{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // an unreadable entry is not a finding
			return nil
		}
		if d.IsDir() {
			if skipped[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !scannedExtensions[filepath.Ext(path)] {
			return nil
		}
		text := string(mustRead(path))
		if strings.Contains(text, exemptionMarker) {
			return nil
		}
		lowered := strings.ToLower(text)
		for _, reporter := range reporters {
			if strings.Contains(lowered, reporter) {
				found = append(found, path+" contains "+reporter)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning: %v", err)
	}
	if len(found) > 0 {
		t.Fatalf("the application must contain no telemetry, but:\n  %s",
			strings.Join(found, "\n  "))
	}
}

// A setting that enables telemetry is telemetry: there is nothing to opt into.
func TestNoTelemetrySettingExists(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("finding the repository: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading the repository: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "service.go") {
			continue
		}
		lowered := strings.ToLower(string(mustRead(filepath.Join(root, entry.Name()))))
		// Words that would mean a setting, not prose about one: a comment
		// explaining why a secret must never reach a crash report is the reason
		// there is no crash reporter, not evidence of one.
		for _, word := range []string{"telemetry", "analytics", "usage data", "crashreport"} {
			if strings.Contains(lowered, word) {
				t.Fatalf("%s offers %q", entry.Name(), word)
			}
		}
	}
}
