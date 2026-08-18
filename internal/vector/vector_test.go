package vector

import (
	"math"
	"testing"
)

// The interesting float32 values: the ones where a format that is nearly right
// stops being right. Negative zero survives only if the bits are copied rather
// than the number; the subnormal survives only if nothing rounds.
var bitPatterns = []float32{
	0,
	float32(math.Copysign(0, -1)),
	1,
	-1,
	math.SmallestNonzeroFloat32,
	math.MaxFloat32,
	-math.MaxFloat32,
	0.1,
	-0.30000001192092896,
	3.4028235e+38,
}

func TestKnownBitPatternsRoundTrip(t *testing.T) {
	blob := Encode(bitPatterns)
	if want := 4 * len(bitPatterns); len(blob) != want {
		t.Fatalf("encoded %d values into %d bytes, want %d", len(bitPatterns), len(blob), want)
	}
	got, err := Decode(blob, len(bitPatterns))
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for i := range bitPatterns {
		// Bits, not values: -0.0 == 0.0 is true and would hide a format that
		// dropped the sign.
		if math.Float32bits(got[i]) != math.Float32bits(bitPatterns[i]) {
			t.Errorf("element %d round-tripped %v (bits %#x) as %v (bits %#x)",
				i, bitPatterns[i], math.Float32bits(bitPatterns[i]),
				got[i], math.Float32bits(got[i]))
		}
	}
}

func TestDecodeRefusesMalformedBlobs(t *testing.T) {
	cases := []struct {
		name string
		blob []byte
		dims int
	}{
		{"a length that is not a multiple of four", make([]byte, 9), 2},
		{"one byte short of a whole vector", make([]byte, 15), 4},
		{"more dimensions than the space has", make([]byte, 16), 3},
		{"fewer dimensions than the space has", make([]byte, 8), 4},
		{"an empty blob in a real space", nil, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Decode(c.blob, c.dims); err == nil {
				t.Fatalf("decoded %d bytes into %d dimensions without complaint", len(c.blob), c.dims)
			}
		})
	}
}

// The oracle: cosine computed independently, by hand, for pairs whose answers
// are known without doing any arithmetic at all.
func TestCosineMatchesATrustedOracle(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"a vector with itself", []float32{1, 2, 3}, []float32{1, 2, 3}, 1},
		{"a vector with its negation", []float32{1, 2, 3}, []float32{-1, -2, -3}, -1},
		{"orthogonal axes", []float32{1, 0}, []float32{0, 1}, 0},
		{"a vector with a scaled copy of itself", []float32{1, 2, 3}, []float32{10, 20, 30}, 1},
		{"forty-five degrees", []float32{1, 0}, []float32{1, 1}, math.Sqrt2 / 2},
		{"one hundred and thirty-five degrees", []float32{1, 0}, []float32{-1, 1}, -math.Sqrt2 / 2},
		// 3·1 + 4·0 = 3, over 5 · 1.
		{"a three-four-five triangle", []float32{3, 4}, []float32{1, 0}, 0.6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Cosine(c.a, c.b)
			if err != nil {
				t.Fatalf("scoring: %v", err)
			}
			if math.Abs(got-c.want) > 1e-9 {
				t.Fatalf("scored %v, want %v", got, c.want)
			}
		})
	}
}

// A score outside [-1, 1] is a number later phases will do arithmetic with
// under the assumption the range is real.
func TestAScoreNeverLeavesTheDefinedRange(t *testing.T) {
	// Enough dimensions that the accumulated error is visible if unclamped.
	long := make([]float32, 1024)
	for i := range long {
		long[i] = float32(i%7) + 0.1
	}
	same, err := Cosine(long, long)
	if err != nil {
		t.Fatalf("scoring: %v", err)
	}
	if same > 1 || same < -1 {
		t.Fatalf("a vector compared with itself scored %v, outside [-1, 1]", same)
	}

	negated := make([]float32, len(long))
	for i, f := range long {
		negated[i] = -f
	}
	opposite, err := Cosine(long, negated)
	if err != nil {
		t.Fatalf("scoring: %v", err)
	}
	if opposite < -1 || opposite > 1 {
		t.Fatalf("a vector compared with its negation scored %v, outside [-1, 1]", opposite)
	}
}

func TestDegenerateVectorsAreRefusedRatherThanScored(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	cases := []struct {
		name string
		v    []float32
	}{
		{"all zeroes", []float32{0, 0, 0}},
		{"a single zero", []float32{0}},
		{"no dimensions at all", []float32{}},
		{"a NaN component", []float32{1, nan, 3}},
		{"a positive infinity", []float32{1, inf, 3}},
		{"a negative infinity", []float32{1, -inf, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := Check(c.v); err == nil {
				t.Fatalf("accepted %v as a storable vector", c.v)
			}
			// And the same refusal arrives through scoring, so a vector that
			// somehow reached storage still cannot produce a number.
			good := make([]float32, len(c.v))
			for i := range good {
				good[i] = 1
			}
			if len(good) > 0 {
				if score, err := Cosine(c.v, good); err == nil {
					t.Fatalf("scored %v against a real vector as %v", c.v, score)
				}
				if score, err := Cosine(good, c.v); err == nil {
					t.Fatalf("scored a real vector against %v as %v", c.v, score)
				}
			}
		})
	}
}

// A result set containing one NaN sorts unpredictably, so the refusal has to
// happen before any comparison rather than being filtered afterwards.
func TestNoScoreIsEverNaN(t *testing.T) {
	nan := float32(math.NaN())
	if _, err := Cosine([]float32{nan, nan}, []float32{nan, nan}); err == nil {
		t.Fatal("two NaN vectors produced a score")
	}
}

func TestMismatchedLengthsAreRefused(t *testing.T) {
	if _, err := Cosine([]float32{1, 2, 3}, []float32{1, 2}); err == nil {
		t.Fatal("compared vectors of different lengths")
	}
}

// float32 accumulation over a long vector loses precision that float64
// accumulation does not; this is the case that separates the two.
func TestAccumulationKeepsPrecisionOverALongVector(t *testing.T) {
	const n = 4096
	a := make([]float32, n)
	b := make([]float32, n)
	for i := range a {
		a[i] = 1
		b[i] = 1
	}
	got, err := Cosine(a, b)
	if err != nil {
		t.Fatalf("scoring: %v", err)
	}
	if math.Abs(got-1) > 1e-12 {
		t.Fatalf("identical long vectors scored %v, want 1", got)
	}
}
