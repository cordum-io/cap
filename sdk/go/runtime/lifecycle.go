package runtime

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func (a *Agent) configureRuntimeDefaults() {
	if a.SenderID == "" {
		a.SenderID = defaultSenderID
	}
	if a.Logger == nil {
		a.Logger = slog.Default()
	}
	if a.Metrics == nil {
		a.Metrics = NoopMetrics
	}
	if a.IOTTimeout == 0 {
		a.IOTTimeout = defaultTimeout
	}
	if a.MaxContextBytes == 0 {
		a.MaxContextBytes = defaultMaxBytes
	}
	if a.MaxResultBytes == 0 {
		a.MaxResultBytes = defaultMaxBytes
	}
}

func (a *Agent) openRuntimeResources() error {
	if a.NATS == nil {
		connector := a.connectNATS
		if connector == nil {
			connector = connectRuntimeNATS
		}
		connection, err := connector(runtimeURL(a.NATSURL, "NATS_URL", defaultNATSURL), a.SenderID, a.IOTTimeout)
		if err != nil {
			return err
		}
		a.NATS, a.ownedNATS = connection, true
	}
	if a.Store == nil {
		opener := a.openStore
		if opener == nil {
			opener = func(url string) (BlobStore, error) { return NewRedisBlobStore(url) }
		}
		store, err := opener(runtimeURL(a.RedisURL, "REDIS_URL", defaultRedisURL))
		if err != nil {
			return err
		}
		a.Store, a.ownedStore = store, true
	}
	return nil
}

func connectRuntimeNATS(url, name string, timeout time.Duration) (NATSConn, error) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return nats.Connect(url, nats.Name(name), nats.Timeout(timeout))
}

func runtimeURL(configured, envName, fallback string) string {
	if value := strings.TrimSpace(configured); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return fallback
}

func (a *Agent) subscribeHandlers() error {
	topics := make([]string, 0, len(a.handlers))
	for topic := range a.handlers {
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	for _, topic := range topics {
		handler := a.handlers[topic]
		subscription, err := a.NATS.QueueSubscribe(topic, topic, func(message *nats.Msg) {
			a.handleMessage(message, handler)
		})
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", topic, err)
		}
		a.subsMu.Lock()
		a.subs = append(a.subs, subscription)
		a.subsMu.Unlock()
	}
	return nil
}

func (a *Agent) failStart(startErr error) error {
	a.stopRenewLoop()
	a.unsubscribeAll()
	if a.ownedNATS {
		closeNATS(a.NATS)
		a.NATS, a.ownedNATS = nil, false
	}
	if a.ownedStore && a.Store != nil {
		_ = a.Store.Close()
		a.Store, a.ownedStore = nil, false
	}
	a.clearSession()
	a.trustMode = ""
	a.trustConfig = nil
	return startErr
}

func (a *Agent) unsubscribeAll() {
	a.subsMu.Lock()
	subscriptions := append([]*nats.Subscription(nil), a.subs...)
	a.subs = nil
	a.subsMu.Unlock()
	for _, subscription := range subscriptions {
		if subscription != nil {
			_ = subscription.Unsubscribe()
		}
	}
}

func closeNATS(connection NATSConn) {
	if native, ok := connection.(*nats.Conn); ok {
		native.Close()
		return
	}
	if closer, ok := connection.(interface{ Close() }); ok {
		closer.Close()
	}
}
