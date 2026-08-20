package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
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
