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

// Working offline is a property of where the code can reach, not a hope.
//
// The PRD's first additional gate is that the recruiter disconnects from the
// internet and keeps using the CRM, artifacts, approved profiles, retrieval,
// Q&A and generation. Proving that fully needs the laptop and a cable pulled
// out. What can be proved from anywhere is the half that would make it
// impossible: a client this repository builds, pointed somewhere that is not
// the local model runtime, that nothing gated it.
//
// So every construction of an HTTP client is confined to two files, and every
// absolute destination in the repository is either the local runtime or a
// remote the recruiter is asked about first. A third file gaining one is the
// thing that breaks offline use, and it fails here on the day it is written
// rather than on the day the cable comes out.
func TestNothingReachesTheNetworkExceptTheRuntimeAndApprovedRemotes(t *testing.T) {
	// Where an outbound client may be built at all. Exa is recruiter-initiated
	// with a per-query preview; the cloud endpoint is configured, approved per
	// task, credentialed, and checked to be the one that was approved; the
	// runtime is local. A fourth file here is a fourth way out.
	allowed := map[string]bool{
		filepath.Join("internal", "platform", "exa.go"):       true,
		filepath.Join("internal", "platform", "ollama.go"):    true,
		filepath.Join("internal", "platform", "cloudchat.go"): true,
	}
	// Destinations that may appear anywhere: the local runtime, and the remotes
	// the PRD names as gated.
	//
	// The cloud endpoint is not here because it has no address in this
	// repository: the recruiter configures one, and the transport is built for
	// whatever they approved. That is the gate — an endpoint nobody typed is an
	// endpoint nothing sends to.
	permitted := []string{
		"http://localhost", "http://127.0.0.1", "https://api.exa.ai",
	}

	builders := []string{"http.NewRequest", "http.Client{", "http.Post(", "http.Get(", "http.Head("}
	offenders := []string{}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && skipped[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body := string(mustRead(path))
		if strings.Contains(body, exemptionMarker) {
			return nil
		}
		clean := filepath.Clean(path)
		for _, builder := range builders {
			if strings.Contains(body, builder) && !allowed[clean] {
				offenders = append(offenders, clean+" builds an outbound client ("+builder+")")
			}
		}
		// And any absolute destination, wherever it is written.
		for _, line := range strings.Split(body, "\n") {
			at := strings.Index(line, "http://")
			if at < 0 {
				at = strings.Index(line, "https://")
			}
			if at < 0 || strings.Contains(line, "//") && strings.TrimSpace(line)[0] == '/' {
				continue
			}
			url := line[at:]
			if cut := strings.IndexAny(url, `"`+" \t)`"); cut > 0 {
				url = url[:cut]
			}
			ok := false
			for _, p := range permitted {
				if strings.HasPrefix(url, p) {
					ok = true
				}
			}
			if !ok {
				offenders = append(offenders, clean+" names "+url)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("the application can reach somewhere it was not gated for:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// A client that carries a credential is built when it is used, not when the
// application starts.
//
// Three defects in a row were this, and all three shipped: the search client
// was built at start-up with an empty key and refused every search the
// recruiter's stored key should have made; the cloud service was handed the
// local runtime's client and answered cloud payloads on this machine; and the
// sidecar was verified before it was installed and refused every document
// afterwards. Each was correct when it was constructed and wrong by the time
// anybody used it.
//
// Nothing caught them, because every test builds its own service with its own
// working dependency. The defect lived only in the wiring, and the wiring had
// no test.
//
// So: start-up may build the local runtime's client, which needs no credential
// and cannot move. It may not build one that carries a key. Those are
// constructed per request, from the store, by the service that sends.
func TestStartUpBuildsNoClientThatCarriesACredential(t *testing.T) {
	body := string(mustRead("main.go"))
	if body == "" {
		t.Fatal("main.go could not be read")
	}

	// Constructors that take a credential. Building one here means holding a
	// secret from before the recruiter has entered it.
	for _, credentialed := range []string{"platform.NewExa(", "platform.NewCloud("} {
		if strings.Contains(body, credentialed) {
			t.Errorf("main.go calls %s — a client built at start-up holds whatever the "+
				"credential was then, which at start-up is nothing", credentialed)
		}
	}

	// The local runtime is the exception, and it is allowed to stay one.
	if !strings.Contains(body, "platform.NewOllama()") {
		t.Error("main.go no longer builds the local runtime client, and this test " +
			"is now guarding a wiring that moved")
	}
}

// There is nothing to turn on.
//
// The spec asks for two things and the network scan above covers one: no
// telemetry request leaves. This is the other — "WHEN the service surface and
// settings are inspected, THEN no telemetry setting, endpoint, or reporter
// exists to enable". A reporter that is present but disabled is one settings
// toggle from being present and enabled, and the toggle is what somebody adds
// when a release is going badly.
//
// It reads the dependency manifests and the identifiers, not prose: the tuning
// corpus contains a marine telemetry engineer, and a scan for the word would
// find him.
//
// telemetry-check-exempt: this file names the vendors to assert their absence
func TestThereIsNoTelemetryToEnable(t *testing.T) {
	vendors := []string{
		"opentelemetry", "go.opentelemetry", "otelhttp", "sentry", "bugsnag",
		"posthog", "mixpanel", "amplitude", "segment.com/analytics",
		"datadog", "newrelic", "rollbar", "crashlytics", "app-insights",
		"applicationinsights", "@vercel/analytics", "google-analytics", "gtag",
	}
	for _, manifest := range []string{"go.mod", "go.sum", filepath.Join("frontend", "package.json")} {
		body := strings.ToLower(string(mustRead(manifest)))
		if body == "" {
			t.Fatalf("%s could not be read", manifest)
		}
		for _, vendor := range vendors {
			if strings.Contains(body, vendor) {
				t.Errorf("%s depends on %s — this application reports on nobody", manifest, vendor)
			}
		}
	}

	// And no identifier offers one: a field, a method, or a setting called
	// telemetry or analytics is a control whether or not anything reads it.
	const marker = "telemetry-check-exempt: this file names the vendors to assert their absence"
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && skipped[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".ts", ".tsx":
		default:
			return nil
		}
		body := string(mustRead(path))
		if strings.Contains(body, marker) {
			return nil
		}
		// Identifiers, capitalised or camel-cased — not the word in a comment
		// or in somebody's job title.
		for _, ident := range []string{"Telemetry", "Analytics", "telemetryEnabled", "analyticsEnabled"} {
			if strings.Contains(body, ident) {
				t.Errorf("%s declares %q, which is a control somebody can enable", path, ident)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
}

// A gate that calls a local model has to be allowed to take as long as the
// model takes.
//
// `go test` gives itself ten minutes by default and panics at that, and one
// classification against a 14B model measured 344 seconds on the development
// machine — so two calls in one binary exceed it. The failure looks exactly
// like a product fault: a stack dump, a timeout, no result. It is a missing
// flag.
//
// This matters most where it cannot be noticed. The target laptop is slower
// than the machine this was written on, so a recipe that fits here may not fit
// there, and the laptop gates are the ones nobody can re-run casually.
func TestEveryLiveModelRecipeBoundsItsOwnRun(t *testing.T) {
	body, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatalf("reading the justfile: %v", err)
	}
	for i, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "go test ") {
			continue
		}
		// Only the ones that call a model or seed a large corpus. The ordinary
		// suite is fast, and the default is a reasonable bound for it.
		if !strings.Contains(trimmed, "-tags livemodel") && !strings.Contains(trimmed, "-tags perf") {
			continue
		}
		if !strings.Contains(trimmed, "-timeout ") {
			t.Errorf("justfile line %d runs a model without bounding the run, so it panics at "+
				"the ten minute default however long the model legitimately takes:\n\t%s",
				i+1, trimmed)
		}
	}
}
