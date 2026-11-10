package a2abridge

// store is the bounded in-memory correlation store.
type store struct {
	max int
}

func newStore(max int) *store {
	return &store{max: max}
}
