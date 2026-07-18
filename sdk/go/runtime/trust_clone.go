package runtime

import (
	"crypto/ecdsa"
	"math/big"

	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

func cloneRuntimeWorkerTrust(config capsdk.WorkerTrustConfig) capsdk.WorkerTrustConfig {
	clone := config
	clone.ProofPrivateKey = cloneRuntimePrivateKey(config.ProofPrivateKey)
	clone.SchedulerPublicKeys = make(map[string]*ecdsa.PublicKey, len(config.SchedulerPublicKeys))
	for keyID, key := range config.SchedulerPublicKeys {
		clone.SchedulerPublicKeys[keyID] = cloneRuntimePublicKey(key)
	}
	return clone
}

func cloneRuntimePrivateKey(key *ecdsa.PrivateKey) *ecdsa.PrivateKey {
	if key == nil {
		return nil
	}
	return &ecdsa.PrivateKey{PublicKey: *cloneRuntimePublicKey(&key.PublicKey), D: cloneRuntimeBigInt(key.D)}
}

func cloneRuntimePublicKey(key *ecdsa.PublicKey) *ecdsa.PublicKey {
	if key == nil {
		return nil
	}
	return &ecdsa.PublicKey{Curve: key.Curve, X: cloneRuntimeBigInt(key.X), Y: cloneRuntimeBigInt(key.Y)}
}

func cloneRuntimeBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}
