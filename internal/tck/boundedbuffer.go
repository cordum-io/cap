package tck

import "sync"

// boundedBuffer captures at most max bytes of a child's stderr for diagnostics
// while never blocking the child: writes past the cap are counted and dropped,
// and Write always reports full consumption so a noisy adapter cannot stall on a
// full pipe. It is safe for concurrent use.
type boundedBuffer struct {
	mu      sync.Mutex
	buf     []byte
	max     int
	dropped int
}

func newBoundedBuffer(max int) *boundedBuffer { return &boundedBuffer{max: max} }

// Write stores up to the cap and discards the rest, always returning len(p) so
// the writing child never sees a short write or error.
func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	room := b.max - len(b.buf)
	if room > 0 {
		if len(p) <= room {
			b.buf = append(b.buf, p...)
		} else {
			b.buf = append(b.buf, p[:room]...)
			b.dropped += len(p) - room
		}
	} else {
		b.dropped += len(p)
	}
	return len(p), nil
}

// String returns the captured prefix.
func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// Dropped returns how many stderr bytes were discarded past the cap.
func (b *boundedBuffer) Dropped() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dropped
}
