package safety

import (
	"math/rand"
	"testing"
	"time"
)

func TestJitterDurationWithinBounds(t *testing.T) {
	j := &Jitterer{
		Min:  45 * time.Second,
		Max:  90 * time.Second,
		Rand: rand.New(rand.NewSource(1)),
	}
	for i := 0; i < 1000; i++ {
		d := j.Duration()
		if d < j.Min || d > j.Max {
			t.Fatalf("duration %v out of [%v,%v]", d, j.Min, j.Max)
		}
	}
}

func TestJitterWaitUsesInjectedSleep(t *testing.T) {
	var slept time.Duration
	j := &Jitterer{
		Min:   45 * time.Second,
		Max:   90 * time.Second,
		Rand:  rand.New(rand.NewSource(2)),
		Sleep: func(d time.Duration) { slept = d },
	}
	got := j.Wait()
	if got != slept {
		t.Fatalf("Wait returned %v but slept %v", got, slept)
	}
	if slept < j.Min || slept > j.Max {
		t.Fatalf("slept %v out of bounds", slept)
	}
}
