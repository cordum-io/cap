package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func handshakeBackoff(attempt int) time.Duration {
	delay := 100 * time.Millisecond
	for index := 0; index < attempt; index++ {
		delay *= 3
	}
	return delay
}

func sleepCtx(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	if ctx == nil {
		time.Sleep(delay)
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
