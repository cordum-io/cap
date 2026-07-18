package worker

import (
	"fmt"
	"strings"
	"time"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

const defaultWorkerTrustTimeout = 10 * time.Second

func validateManagedTrustConfig(config ManagedConfig, resolvedWorkerID string) error {
	switch config.WorkerTrustMode {
	case capsdk.WorkerTrustModeOff:
		if config.WorkerTrust != nil || config.WorkerTrustTimeout != 0 {
			return fmt.Errorf("worker trust: off mode cannot retain trust configuration")
		}
		return nil
	case capsdk.WorkerTrustModeWarn, capsdk.WorkerTrustModeEnforce:
		if err := capsdk.ValidateWorkerTrustConfig(config.WorkerTrust); err != nil {
			return fmt.Errorf("worker trust: %w", err)
		}
		if strings.TrimSpace(resolvedWorkerID) != config.WorkerTrust.WorkerID {
			return fmt.Errorf("worker trust: worker id does not match pinned identity")
		}
		if config.WorkerTrustTimeout < 0 {
			return fmt.Errorf("worker trust: timeout must not be negative")
		}
		if config.WorkerTrustTimeout > capsdk.WorkerHandshakeMaxLifetime {
			return fmt.Errorf("worker trust: timeout exceeds %s", capsdk.WorkerHandshakeMaxLifetime)
		}
		return nil
	default:
		return fmt.Errorf("worker trust: %w: %s", capsdk.ErrWorkerTrustMode, config.WorkerTrustMode)
	}
}

func managedTrustTimeout(config ManagedConfig) time.Duration {
	if config.WorkerTrustTimeout > 0 {
		return config.WorkerTrustTimeout
	}
	return defaultWorkerTrustTimeout
}
