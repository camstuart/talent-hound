package vector

import (
	"fmt"
	"testing"
	"time"
)

// spread generates the corpus. It is a plain multiplicative generator rather
// than math/rand because the numbers only have to be varied and identical on
// every machine — a seeded library generator would be the same thing with a
// dependency and a security-linter argument attached.
type spread struct{ state uint64 }

func (s *spread) next() float32 {
	s.state = s.state*6364136223846793005 + 1442695040888963407
	// Top bits, scaled to roughly [-1, 1).
	return float32(s.state>>40)/(1<<23) - 1
}

// The PRD leaves the exact scan in place unless it is measured and found
// wanting, which means the numbers have to exist. This is the measurement: a
// full cosine scan over an in-memory corpus at increasing sizes, printed as
// EVIDENCE lines for the gate record.
//
// It asserts a generous ceiling rather than a tight one. The point is to catch
// the day exact scanning stops being viable, not to fail on a busy CI machine.
const scanCeiling = 250 * time.Millisecond

func TestExactScanCostAtIncreasingCorpusSizes(t *testing.T) {
	// Representative of a real embedding model's output width.
	const dims = 1024
	// A realistic ceiling for one initiative's evidence: a few thousand chunks
	// is a few hundred documents.
	sizes := []int{100, 1_000, 5_000, 20_000}

	// Fixed seed: the numbers must be comparable between runs and machines.
	rng := &spread{state: 42}
	makeVec := func() []float32 {
		v := make([]float32, dims)
		for i := range v {
			v[i] = rng.next()
		}
		return v
	}
	query := makeVec()

	for _, n := range sizes {
		corpus := make([][]float32, n)
		for i := range corpus {
			corpus[i] = makeVec()
		}

		start := time.Now()
		best := -2.0
		for _, v := range corpus {
			score, err := Cosine(query, v)
			if err != nil {
				t.Fatalf("scoring at size %d: %v", n, err)
			}
			if score > best {
				best = score
			}
		}
		elapsed := time.Since(start)

		// Paste this line into the gate evidence. Under the race detector it is
		// labelled, because an instrumented timing is not a timing.
		build := "plain"
		if raceEnabled {
			build = "race-instrumented"
		}
		fmt.Printf("EVIDENCE exact-scan corpus=%d dims=%d per_query=%s per_vector=%s build=%s\n",
			n, dims, elapsed.Round(time.Microsecond),
			(elapsed / time.Duration(n)).Round(time.Nanosecond), build)

		if elapsed > scanCeiling && !raceEnabled {
			t.Errorf("a scan over %d vectors of %d dimensions took %s, above the %s ceiling "+
				"at which an approximate index becomes worth its correctness risk",
				n, dims, elapsed, scanCeiling)
		}
	}
}

// The same measurement as a benchmark, for anyone wanting per-op figures rather
// than a pass or a fail.
func BenchmarkCosine(b *testing.B) {
	const dims = 1024
	rng := &spread{state: 42}
	mk := func() []float32 {
		v := make([]float32, dims)
		for i := range v {
			v[i] = rng.next()
		}
		return v
	}
	x, y := mk(), mk()
	for b.Loop() {
		if _, err := Cosine(x, y); err != nil {
			b.Fatal(err)
		}
	}
}
