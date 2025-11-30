package pkg

import (
	"math/rand"
	"sync"
	"time"
)

type LockedRand struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func NewLockedRand() *LockedRand {
	return &LockedRand{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (r *LockedRand) Intn(n int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rng.Intn(n)
}

func (r *LockedRand) Shuffle(n int, swap func(i, j int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rng.Shuffle(n, swap)
}
