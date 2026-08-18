//go:build !race

package vector

// raceEnabled is false in an ordinary build, where a timing is a timing.
const raceEnabled = false
