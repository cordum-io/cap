package worker

import (
	"crypto/ecdsa"
	"math/big"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func cloneManagedTrustConfig(config ManagedConfig) ManagedConfig {
	config.WorkerTrust = cloneWorkerTrustConfig(config.WorkerTrust)
	return config
}

func cloneWorkerTrustConfig(config *capsdk.WorkerTrustConfig) *capsdk.WorkerTrustConfig {
	if config == nil {
		return nil
	}
	clone := *config
	clone.ProofPrivateKey = cloneECDSAPrivateKey(config.ProofPrivateKey)
	clone.SchedulerPublicKeys = make(map[string]*ecdsa.PublicKey, len(config.SchedulerPublicKeys))
	for keyID, key := range config.SchedulerPublicKeys {
		clone.SchedulerPublicKeys[keyID] = cloneECDSAPublicKey(key)
	}
	return &clone
}

func cloneECDSAPrivateKey(key *ecdsa.PrivateKey) *ecdsa.PrivateKey {
	if key == nil {
		return nil
	}
	clone := &ecdsa.PrivateKey{PublicKey: *cloneECDSAPublicKey(&key.PublicKey)}
	clone.D = cloneBigInt(key.D)
	return clone
}

func cloneECDSAPublicKey(key *ecdsa.PublicKey) *ecdsa.PublicKey {
	if key == nil {
		return nil
	}
	return &ecdsa.PublicKey{Curve: key.Curve, X: cloneBigInt(key.X), Y: cloneBigInt(key.Y)}
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}
