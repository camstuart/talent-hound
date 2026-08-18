// Package vector is the numerical half of semantic retrieval: how a vector
// becomes bytes, how bytes become a vector again, and what one vector's
// similarity to another is.
//
// It knows nothing about embedding spaces on purpose. Establishing that two
// vectors mean the same thing is the caller's job, and it is a database
// question rather than an arithmetic one — see the embedding service, which is
// the only thing allowed to decide that two vectors may be compared.
package vector

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Encode serializes v as little-endian float32, with no header and no padding:
// N dimensions occupy exactly 4N bytes.
//
// The format carries no length or version because it never travels alone — it
// is a column in a row that names its embedding space, and the space carries
// the dimension count. A blob that does not match is refused at the boundary
// rather than reinterpreted.
func Encode(v []float32) []byte {
	out := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(f))
	}
	return out
}

// Decode reverses Encode, bit for bit. dims is the dimension count of the
// embedding space the blob is being read for; a blob of any other size is an
// error rather than something to truncate or pad.
func Decode(b []byte, dims int) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("a vector blob of %d bytes is not a whole number of float32 values", len(b))
	}
	n := len(b) / 4
	if n != dims {
		return nil, fmt.Errorf("a vector blob of %d dimensions cannot be read in a space of %d", n, dims)
	}
	out := make([]float32, n)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out, nil
}

// Check reports whether v is a vector that can be scored: finite in every
// component and with a magnitude to have a direction.
//
// An all-zero vector points nowhere, so its similarity to anything is
// undefined; a non-finite component poisons the magnitude and turns every score
// in a result set into NaN, which sorts unpredictably. Both are failures of the
// model that produced them, and are refused before storage rather than
// discovered at query time.
func Check(v []float32) error {
	if len(v) == 0 {
		return fmt.Errorf("a vector with no dimensions is not a vector")
	}
	zero := true
	for i, f := range v {
		f64 := float64(f)
		if math.IsNaN(f64) || math.IsInf(f64, 0) {
			return fmt.Errorf("dimension %d is not a finite number", i)
		}
		if f != 0 {
			zero = false
		}
	}
	if zero {
		return fmt.Errorf("a vector of all zeroes has no direction to compare")
	}
	return nil
}

// Cosine returns the cosine similarity of a and b, in the range -1 to 1.
//
// Accumulation is float64 over float32 inputs: summing a thousand float32
// products in float32 loses more precision than the extra byte costs. The
// result is clamped because floating-point accumulation can hand back
// 1.0000000000000002 for a vector compared with itself, and later phases do
// arithmetic that assumes the range is real.
func Cosine(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("cannot compare a vector of %d dimensions with one of %d", len(a), len(b))
	}
	if err := Check(a); err != nil {
		return 0, fmt.Errorf("left vector: %w", err)
	}
	if err := Check(b); err != nil {
		return 0, fmt.Errorf("right vector: %w", err)
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	return clamp(dot / (math.Sqrt(na) * math.Sqrt(nb))), nil
}

func clamp(f float64) float64 {
	return math.Max(-1, math.Min(1, f))
}
