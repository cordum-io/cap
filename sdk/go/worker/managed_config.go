package worker

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
)

type resolvedManagedConfig struct {
	subjects    []string
	admitted    []string
	workerID    string
	pool        string
	queue       string
	natsURL     string
	maxParallel int32
	logger      *log.Logger
}

func resolveManagedConfig(config ManagedConfig) (resolvedManagedConfig, error) {
	subjects := trimSubjects(config.Subjects)
	if len(subjects) == 0 {
		workerType := strings.TrimSpace(config.Type)
		if workerType == "" {
			return resolvedManagedConfig{}, errors.New("subjects required")
		}
		subjects = []string{fmt.Sprintf("job.%s.*", workerType)}
	}
	workerID := resolveWorkerID(config.WorkerID, config.Type)
	if err := validateManagedTrustConfig(config, workerID); err != nil {
		return resolvedManagedConfig{}, err
	}
	resolved := resolvedManagedConfig{
		subjects: subjects, admitted: managedAdmittedSubjects(workerID, subjects),
		workerID: workerID, pool: strings.TrimSpace(config.Pool), queue: strings.TrimSpace(config.Queue),
		natsURL: strings.TrimSpace(config.NatsURL), maxParallel: config.MaxParallelJobs, logger: config.Logger,
	}
	applyManagedDefaults(&resolved, config.Type)
	return resolved, nil
}

func applyManagedDefaults(config *resolvedManagedConfig, workerType string) {
	if config.pool == "" {
		config.pool = strings.TrimSpace(workerType)
	}
	if config.natsURL == "" {
		config.natsURL = strings.TrimSpace(os.Getenv("NATS_URL"))
	}
	if config.natsURL == "" {
		config.natsURL = defaultNATSURL
	}
	if config.maxParallel <= 0 {
		config.maxParallel = defaultMaxParallel
	}
	if config.logger == nil {
		config.logger = log.New(os.Stdout, "cap-worker ", log.LstdFlags)
	}
}

func connectManagedNATS(config ManagedConfig, resolved resolvedManagedConfig) (*nats.Conn, error) {
	options := []nats.Option{nats.Name(resolved.workerID), nats.Timeout(defaultConnectTimeout)}
	tlsConfig := config.NATSTLSConfig
	if tlsConfig == nil {
		var err error
		tlsConfig, err = capsdk.NATSTLSConfigFromEnv()
		if err != nil {
			return nil, fmt.Errorf("nats tls config: %w", err)
		}
	}
	if tlsConfig != nil {
		options = append(options, nats.Secure(tlsConfig))
	}
	return nats.Connect(resolved.natsURL, options...)
}
