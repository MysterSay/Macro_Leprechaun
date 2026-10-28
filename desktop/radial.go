package main

import "math"

// radialIndex selects an angular sector clockwise from twelve o'clock.
// The center is a dead zone; distance from a particular button is irrelevant.
func radialIndex(dx, dy float64, count int) int {
	if count == 0 || math.Hypot(dx, dy) < 52 {
		return -1
	}
	a := math.Atan2(dy, dx) + math.Pi/2
	if a < 0 {
		a += 2 * math.Pi
	}
	return int(math.Floor(a/(2*math.Pi/float64(count))+.5)) % count
}
