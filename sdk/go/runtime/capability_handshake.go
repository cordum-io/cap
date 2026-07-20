package runtime

import (
	"sort"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"google.golang.org/protobuf/proto"
)

func (a *Agent) buildCapabilityHandshake() *agentv1.Handshake {
	capabilities := make(map[string]bool, len(a.handlers))
	readyTopics := make([]string, 0, len(a.handlers))
	for topic := range a.handlers {
		capabilities[topic] = true
		readyTopics = append(readyTopics, topic)
	}
	sort.Strings(readyTopics)
	return &agentv1.Handshake{
		ComponentId:       a.SenderID,
		Role:              agentv1.ComponentRole_COMPONENT_ROLE_WORKER,
		SupportedVersions: []int32{capsdk.DefaultProtocolVersion},
		Capabilities:      capabilities,
		SdkVersion:        a.effectiveSDKVersion(),
		ReadyTopics:       readyTopics,
		AgentName:         capsdk.SanitizeAgentName(a.AgentName),
	}
}

func (a *Agent) capabilityHandshake() *agentv1.Handshake {
	if a.capability == nil {
		a.capability = a.buildCapabilityHandshake()
	}
	return proto.Clone(a.capability).(*agentv1.Handshake)
}
