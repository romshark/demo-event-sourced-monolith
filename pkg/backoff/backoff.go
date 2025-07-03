// Package backoff provides a calculator for backoff with jitter.
package backoff

import (
	"fmt"
	"math"
	"time"
)

// RandReader provides https://pkg.go.dev/math/rand/v2#Float64.
type RandReader interface{ Float64() float64 }

// Backoff provides exponential backoff with jitter.
type Backoff struct {
	Min        time.Duration // Minimum backoff duration (must be greater 0 and Max).
	Max        time.Duration // Maximum backoff duration.
	Factor     float64       // Exponential growth factor. Must be greater 1.0.
	Jitter     float64       // Jitter ratio in [0.0, 1.0]
	RandSource RandReader
}

// New checks the parameters and returns a new backoff if they're correct,
// otherwise returns an error.
func New(
	min, max time.Duration, factor, jitter float64, randSource RandReader,
) (Backoff, error) {
	if min <= 0 {
		return Backoff{}, fmt.Errorf("min(%d) must be >0", min)
	}
	if min > max {
		return Backoff{}, fmt.Errorf("min(%s) > max(%s)", min, max)
	}
	if factor <= 1.0 {
		return Backoff{}, fmt.Errorf("factor(%g) must be >1.0", factor)
	}
	if jitter < 0 || jitter > 1 {
		return Backoff{}, fmt.Errorf("jitter(%g) must be >=0.0 && <=1.0", jitter)
	}
	return Backoff{
		Min:        min,
		Max:        max,
		Factor:     factor,
		Jitter:     jitter,
		RandSource: randSource,
	}, nil
}

// Duration returns the backoff delay for attempt.
func (b Backoff) Duration(attempt int) time.Duration {
	exp := float64(b.Min) * math.Pow(b.Factor, float64(attempt))
	d := min(time.Duration(exp), b.Max)
	if b.Jitter == 0 {
		return d
	}
	randomJitterFactor := b.RandSource.Float64()*2 - 1 // In [-1.0, 1.0]
	delta := float64(d) * b.Jitter * randomJitterFactor
	return max(d+time.Duration(delta), b.Min)
}
