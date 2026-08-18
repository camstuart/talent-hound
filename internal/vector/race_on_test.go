//go:build race

package vector

// raceEnabled tells the scan measurement not to judge its own numbers.
//
// The race detector instruments every memory access and costs the better part
// of an order of magnitude. The figures are still printed under it — they are
// just not evidence of anything about the shipped build.
const raceEnabled = true
