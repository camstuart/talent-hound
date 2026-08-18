// Command fakesidecar misbehaves on demand so the containment gate tests can
// prove Job Object limits. Built by the tests, never shipped.
// ponytail: one binary with modes instead of four binaries.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "ok":
		fmt.Print("# ok\n")
	case "hang":
		time.Sleep(10 * time.Minute)
	case "spawn":
		// Spawn a grandchild that outlives us, record its pid in the file named
		// by the second argument, then hang.
		child := exec.Command(os.Args[0], "hang")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if len(os.Args) > 2 {
			pid := fmt.Sprintf("%d", child.Process.Pid)
			if err := os.WriteFile(os.Args[2], []byte(pid), 0o600); err != nil {
				os.Exit(3)
			}
		}
		time.Sleep(10 * time.Minute)
	case "alloc":
		var hold [][]byte
		for {
			b := make([]byte, 16<<20)
			for i := range b {
				b[i] = 1 // touch pages so they are committed
			}
			hold = append(hold, b)
		}
	case "flood":
		line := strings.Repeat("x", 1<<16) + "\n"
		for {
			if _, err := os.Stdout.WriteString(line); err != nil {
				return
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", mode)
		os.Exit(1)
	}
}
