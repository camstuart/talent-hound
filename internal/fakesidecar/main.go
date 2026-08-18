// Command fakesidecar stands in for the packaged MarkItDown sidecar in tests.
//
// Containment cannot be tested against a mock: a Job Object kills processes,
// not function calls, so proving that a hang is terminated or that a spawned
// child does not outlive its parent needs a program that really hangs and
// really spawns. This is that program. It is built by the tests that need it
// and is not part of the application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// version is what --version reports. The tests override it to prove the pinned
// version check refuses a sidecar that is not ours.
func version() string {
	if v := os.Getenv("FAKE_SIDECAR_VERSION"); v != "" {
		return v
	}
	return "0.1.2"
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--version" {
		fmt.Println("markitdown", version())
		return
	}

	if len(args) > 0 && args[0] == "--hang" {
		select {}
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: fakesidecar <input>")
		os.Exit(64)
	}
	body, err := os.ReadFile(args[0]) // #nosec G304,G703 -- the contract is "one input path", and this program is only ever run by tests
	if err != nil {
		os.Exit(4)
	}

	// Behaviour is chosen by a marker in the document, not by the file name:
	// the staging path deliberately carries no identity, so there is nothing in
	// the name to dispatch on — which is the point.
	switch mode(body) {
	case "hang":
		select {}
	case "flood":
		line := strings.Repeat("x", 1024) + "\n"
		for i := 0; i < 20*1024; i++ {
			fmt.Print(line)
		}
	case "child":
		// A child that outlives its parent, which only a job object stops.
		self, err := os.Executable()
		if err != nil {
			os.Exit(3)
		}
		// #nosec G204 -- self is this program's own path, with a fixed argument.
		cmd := exec.CommandContext(context.Background(), self, "--hang")
		if err := cmd.Start(); err != nil {
			os.Exit(3)
		}
		fmt.Printf("# spawned %d\n", cmd.Process.Pid)
		time.Sleep(50 * time.Millisecond)
	case "fail":
		// Deliberately quoting the "document": the caller must store none of it.
		fmt.Fprintln(os.Stderr, "ParseError at offset 41: unexpected token in 'Priya Raman, Staff Engineer'")
		os.Exit(2)
	case "empty":
		return
	default:
		fmt.Printf("# %s\n\n%s\n", filepath.Base(args[0]), body)
	}
}

// mode reads the FAKE: marker the tests embed in their fixtures.
func mode(body []byte) string {
	const marker = "FAKE:"
	head := body
	if len(head) > 512 {
		head = head[:512]
	}
	i := strings.Index(string(head), marker)
	if i < 0 {
		return ""
	}
	rest := string(head[i+len(marker):])
	return strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0])
}
