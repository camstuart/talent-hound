// Command hashcheck prints the corpus hash from a separate process.
//
// Go randomizes map iteration, so a hash that is stable within one process can
// still be unstable across runs. The only way to see that is to run it twice,
// in two processes.
package main

import (
	"fmt"
	"os"

	"camstuart/talent-hound/internal/bench"
)

func main() {
	hash, err := bench.Hash()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
