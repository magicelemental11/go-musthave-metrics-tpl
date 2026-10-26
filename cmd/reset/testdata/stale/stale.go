package stale

type T struct {
	A int
}

// Reset is hand written.
func (t *T) Reset() {
	t.A = 0
}
