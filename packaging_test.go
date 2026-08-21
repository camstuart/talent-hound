package main

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/extract"
)

// The identity a recruiter sees before they see the application: the installer,
// the Start menu entry, the uninstall entry, and the executable's properties.
//
// Wails scaffolds these as "My Product" by "My Company" at version 0.0.1, and
// nothing in a routine run reads them — they are consumed on a machine none of
// this suite runs on, which is how a product ships under the template's name.

func TestPackagingMetadataIsTheProductsOwn(t *testing.T) {
	config, err := os.ReadFile("build/config.yml")
	if err != nil {
		t.Fatalf("reading build/config.yml: %v", err)
	}
	// Live settings only: the file ships with a commented iOS example that
	// carries the scaffold names on purpose, and a commented line configures
	// nothing.
	live := []string{}
	for _, line := range strings.Split(string(config), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		live = append(live, line)
	}
	text := strings.Join(live, "\n")
	for _, scaffold := range []string{
		"My Company", "My Product", "com.mycompany", "A program that does X",
		"Some Product Comments", `version: "0.0.1"`,
	} {
		if strings.Contains(text, scaffold) {
			t.Fatalf("build/config.yml still carries the scaffold value %q", scaffold)
		}
	}
	for _, required := range []string{"Talent Hound", "camstuart"} {
		if !strings.Contains(text, required) {
			t.Fatalf("build/config.yml does not name %q", required)
		}
	}
}

func TestTheWindowsFileInfoIsTheProductsOwn(t *testing.T) {
	raw, err := os.ReadFile("build/windows/info.json")
	if err != nil {
		t.Fatalf("reading build/windows/info.json: %v", err)
	}
	var info struct {
		Fixed struct {
			FileVersion string `json:"file_version"`
		} `json:"fixed"`
		Info map[string]map[string]string `json:"info"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatalf("reading build/windows/info.json: %v", err)
	}
	block, ok := info.Info["0000"]
	if !ok {
		t.Fatal("build/windows/info.json has no version block")
	}
	for field, value := range block {
		if strings.Contains(value, "My Company") || value == "This is a comment" {
			t.Fatalf("%s still carries a scaffold value: %q", field, value)
		}
	}
	if block["ProductName"] != "Talent Hound" {
		t.Fatalf("ProductName is %q", block["ProductName"])
	}
}

// The installer's version and the version the application reports have to be
// the same release. A recruiter reading "0.1.0" in Add/Remove Programs and
// "0.1.0-poc" in the diagnostics is reading one release, and a diagnostic
// report that disagrees with the installer is a report nobody can act on.
func TestTheInstallerVersionMatchesTheApplication(t *testing.T) {
	raw, err := os.ReadFile("build/windows/info.json")
	if err != nil {
		t.Fatalf("reading build/windows/info.json: %v", err)
	}
	var info struct {
		Fixed struct {
			FileVersion string `json:"file_version"`
		} `json:"fixed"`
		Info map[string]map[string]string `json:"info"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatalf("reading: %v", err)
	}

	// The file version is numeric by Windows' rules; the application may carry a
	// qualifier after it.
	numeric, _, _ := strings.Cut(Version, "-")
	if info.Fixed.FileVersion != numeric {
		t.Fatalf("the installer says %q and the application says %q",
			info.Fixed.FileVersion, Version)
	}
	if got := info.Info["0000"]["ProductVersion"]; got != numeric {
		t.Fatalf("ProductVersion is %q and the application says %q", got, Version)
	}

	config, err := os.ReadFile("build/config.yml")
	if err != nil {
		t.Fatalf("reading build/config.yml: %v", err)
	}
	if !strings.Contains(string(config), `version: "`+numeric+`"`) {
		t.Fatalf("build/config.yml does not carry version %q", numeric)
	}
}

// The uninstaller runs on a machine this suite never runs on, and it is the one
// place a mistake destroys everything a recruiter holds. On Windows the data
// folder is %AppData%\talent-hound, and the WebView2 directory the uninstaller
// does remove is %AppData%\talent-hound.exe — four characters apart, in the
// same parent.
func TestTheUninstallerKeepsTheDataFolderAndSaysWhereItIs(t *testing.T) {
	raw, err := os.ReadFile("build/windows/nsis/project.nsi")
	if err != nil {
		t.Fatalf("reading the installer script: %v", err)
	}
	script := string(raw)
	start := strings.Index(script, `Section "uninstall"`)
	if start < 0 {
		t.Fatal("the installer script has no uninstall section")
	}
	uninstall := script[start:]

	// Nothing may recursively remove the data folder. The WebView2 directory is
	// named for the executable and keeps its suffix.
	for _, line := range strings.Split(uninstall, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "RMDir") {
			continue
		}
		if !strings.Contains(trimmed, "$AppData") {
			continue
		}
		if !strings.Contains(trimmed, "PRODUCT_EXECUTABLE") {
			t.Fatalf("the uninstaller removes an AppData folder that is not the WebView2 one: %q", trimmed)
		}
	}

	// And it says where the data is, rather than leaving the recruiter to
	// wonder whether it went with the application.
	for _, required := range []string{"data folder", "INFO_PROJECTNAME", "Credential Manager"} {
		if !strings.Contains(uninstall, required) {
			t.Fatalf("the uninstaller never mentions %q", required)
		}
	}
	if !strings.Contains(uninstall, "DetailPrint") {
		t.Fatal("the uninstaller says nothing in a silent uninstall's log")
	}
}

// The version the sidecar is pinned to and the version the application demands
// of it are the same number in two files. When they drift, the packaged reader
// is rejected on first run — on the laptop, by an application that will not
// extract a single document until someone works out why.
func TestTheSidecarPinMatchesWhatTheApplicationDemands(t *testing.T) {
	raw, err := os.ReadFile("build/sidecar/requirements.txt")
	if err != nil {
		t.Fatalf("reading the sidecar requirements: %v", err)
	}
	pinned := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "markitdown") {
			continue
		}
		_, version, ok := strings.Cut(trimmed, "==")
		if !ok {
			t.Fatalf("markitdown is not pinned to an exact version: %q", trimmed)
		}
		pinned = strings.TrimSpace(version)
	}
	if pinned == "" {
		t.Fatal("the sidecar requirements do not pin markitdown")
	}
	if pinned != extract.PinnedSidecarVersion {
		t.Fatalf("requirements.txt pins markitdown %s and the application demands %s",
			pinned, extract.PinnedSidecarVersion)
	}

	// And the record of what was built names the same version.
	pin, err := os.ReadFile("build/sidecar/PIN.md")
	if err != nil {
		t.Fatalf("reading the pin record: %v", err)
	}
	if !strings.Contains(string(pin), pinned) {
		t.Fatalf("PIN.md does not record version %s", pinned)
	}
	// A pin record that sends someone to a command that does not exist is worse
	// than one that says nothing, on the day they have the laptop.
	for _, referenced := range referencedRecipes(string(pin)) {
		if !hasRecipe(t, referenced) {
			t.Fatalf("PIN.md tells the reader to run `just %s`, which does not exist", referenced)
		}
	}
}

// referencedRecipes finds every `just <name>` a document tells the reader to run.
func referencedRecipes(text string) []string {
	out := []string{}
	// Everything after a `just — the first segment is the text before the first
	// one, and reading it as a reference finds whatever backtick came earlier.
	segments := strings.Split(text, "`just ")
	for _, part := range segments[1:] {
		if !strings.Contains(part, "`") {
			continue
		}
		name, _, _ := strings.Cut(part, "`")
		name = strings.TrimSpace(strings.SplitN(name, " ", 2)[0])
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func recipeNames(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatalf("reading the justfile: %v", err)
	}
	names := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		name, _, _ := strings.Cut(line, ":")
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func hasRecipe(t *testing.T, name string) bool {
	t.Helper()
	for _, recipe := range recipeNames(t) {
		if recipe == name {
			return true
		}
	}
	return false
}

// The laptop acceptance run has one entry point, and it names every gate that
// cannot run anywhere else.
//
// The recipe prints its manual checklist by reading the evidence document, so
// the two cannot drift — but it can only do that while the recipe exists and
// the document still has the tables it reads. Both are checked here, because
// the day this matters is the day someone has the laptop and no time.
func TestTheLaptopAcceptanceRunHasOneEntryPoint(t *testing.T) {
	recipes := recipeNames(t)
	if !hasRecipe(t, "laptop-gates") {
		t.Fatal("there is no laptop-gates recipe")
	}
	raw, err := os.ReadFile("justfile")
	if err != nil {
		t.Fatalf("reading the justfile: %v", err)
	}
	body := string(raw)
	start := strings.Index(body, "\nlaptop-gates:")
	if start < 0 {
		t.Fatal("laptop-gates is listed but has no body")
	}
	recipe := body[start:]
	if end := strings.Index(recipe[1:], "\n\n# "); end > 0 {
		recipe = recipe[:end]
	}

	// Every gate that needs Windows or the real models has to be reachable from
	// it, or it is not the entry point it claims to be.
	for _, needed := range []string{"check", "gate", "sidecar", "gate-model",
		"gate-model-classify", "bench", "bench-assess"} {
		if !strings.Contains(recipe, "just "+needed+"\n") {
			t.Errorf("laptop-gates never runs `just %s`", needed)
		}
		if !slices.Contains(recipes, needed) {
			t.Errorf("laptop-gates runs `just %s`, which does not exist", needed)
		}
	}
	if !strings.Contains(recipe, "wails3 task package") {
		t.Error("laptop-gates never packages the application")
	}

	// And the document it reads its checklist out of still has one to read.
	evidence, err := os.ReadFile("docs/product/PHASE20_PACKAGING_EVIDENCE.md")
	if err != nil {
		t.Fatalf("reading the packaging evidence: %v", err)
	}
	rows := 0
	for _, line := range strings.Split(string(evidence), "\n") {
		if strings.HasPrefix(line, "| ") && !strings.HasPrefix(line, "| Check") &&
			!strings.HasPrefix(line, "| ---") {
			rows++
		}
	}
	if rows < 20 {
		t.Fatalf("the packaging evidence has %d checklist rows, which is too few to be the record", rows)
	}
}
