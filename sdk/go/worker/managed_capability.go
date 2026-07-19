package worker

import (
	"fmt"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func buildManagedCapability(config ManagedConfig, workerID string, subjects []string) *agentv1.Handshake {
	sdkVersion := ""
	if config.WorkerTrust != nil {
		sdkVersion = config.WorkerTrust.SDKVersion
	}
	return &agentv1.Handshake{
		ComponentId: workerID, Role: agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
		SupportedVersions: []int32{capsdk.DefaultProtocolVersion},
		Capabilities:      managedCapabilities(config.Capabilities),
		SdkVersion:        sdkVersion,
		ReadyTopics:       append([]string(nil), subjects...),
		AgentName:         capsdk.SanitizeAgentName(config.AgentName),
	}
}

func managedCapabilities(values []string) map[string]bool {
	capabilities := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			capabilities[value] = true
		}
	}
	return capabilities
}

func (w *ManagedWorker) capabilityClone() *agentv1.Handshake {
	if w.capability == nil {
		return nil
	}
	return proto.Clone(w.capability).(*agentv1.Handshake)
}

func (w *ManagedWorker) managedTraceID(kind string) string {
	return fmt.Sprintf("%s-%s-%d", w.workerID, kind, time.Now().UnixNano())
}

func (w *ManagedWorker) sessionToken() string {
	if w.trust == nil || w.trust.mode == capsdk.WorkerTrustModeOff {
		return ""
	}
	return w.trust.token(time.Now())
}

func (w *ManagedWorker) privilegedSessionToken() (string, error) {
	if w.trust == nil || w.trust.mode == capsdk.WorkerTrustModeOff {
		return "", nil
	}
	token := w.trust.token(time.Now())
	if token == "" {
		return "", fmt.Errorf("worker trust: privileged publish requires a live session")
	}
	return token, nil
}

func managedAdmittedSubjects(workerID string, configured []string) []string {
	subjects := append([]string(nil), configured...)
	direct := DirectSubject(workerID)
	for _, subject := range subjects {
		if subject == direct {
			return subjects
		}
	}
	if direct != "" {
		subjects = append(subjects, direct)
	}
	return subjects
}
