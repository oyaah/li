package safety

import (
	"math/rand"
	"time"
)

// Jitterer sleeps a uniform random delay before write actions, to avoid the
// machine-gun timing pattern that LinkedIn flags. Rand and Sleep are injectable
// for deterministic, instant tests.
type Jitterer struct {
	Min   time.Duration
	Max   time.Duration
	Rand  *rand.Rand
	Sleep func(time.Duration)
}

// NewJitter returns a 45–90s jitterer wired to the real clock.
func NewJitter() *Jitterer {
	return &Jitterer{
		Min:   45 * time.Second,
		Max:   90 * time.Second,
		Rand:  rand.New(rand.NewSource(time.Now().UnixNano())),
		Sleep: time.Sleep,
	}
}

// Duration returns a delay in [Min, Max].
func (j *Jitterer) Duration() time.Duration {
	span := int64(j.Max - j.Min)
	if span <= 0 {
		return j.Min
	}
	return j.Min + time.Duration(j.Rand.Int63n(span+1))
}

// Wait sleeps for a jittered duration and returns how long it slept.
func (j *Jitterer) Wait() time.Duration {
	d := j.Duration()
	j.Sleep(d)
	return d
}
