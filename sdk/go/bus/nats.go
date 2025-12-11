package bus

import (
	"time"

	"github.com/nats-io/nats.go"
)

// Config holds NATS connection settings.
type Config struct {
	URL           string
	Token         string
	Username      string
	Password      string
	ConnectTimeout time.Duration
}

// Connect creates a NATS connection with sane defaults.
func Connect(cfg Config) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("cap-sdk-go"),
	}
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}
	if cfg.Username != "" || cfg.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.Username, cfg.Password))
	}
	if cfg.ConnectTimeout > 0 {
		opts = append(opts, nats.Timeout(cfg.ConnectTimeout))
	}
	return nats.Connect(cfg.URL, opts...)
}
