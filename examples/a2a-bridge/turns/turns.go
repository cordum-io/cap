// Package turns is the turn logic of the example A2A bridge worker
// (examples/a2a-bridge/worker). It is importable so that the bridge's
// end-to-end test runs exactly this logic. Inputs and results follow
// spec/interop/a2a-mapping.md §4.
package turns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/cordum-io/cap/v2/sdk/go/client"
	"github.com/cordum-io/cap/v2/sdk/go/runtime"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MediaType identifies the turn-result envelope (spec §4). It is a worker-side
// copy of the bridge's constant; a worker never imports the bridge.
const MediaType = "application/vnd.cordum.cap.a2a-turn-result.v1+json"

var idRe = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Part is one part of the turn input.
type Part struct {
	Kind      string          `json:"kind"`
	Text      *string         `json:"text,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	MediaType string          `json:"media_type,omitempty"`
}

// Input is the turn input the bridge stores at ctx:<job_id>.
type Input struct {
	TaskID    string `json:"task_id"`
	ContextID string `json:"context_id"`
	Turn      int    `json:"turn"`
	MessageID string `json:"message_id"`
	Parts     []Part `json:"parts"`
}

// TextPart is a text part of the envelope.
type TextPart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Message is the envelope's optional status message.
type Message struct {
	Parts []TextPart `json:"parts"`
}

// Artifact is one envelope artifact.
type Artifact struct {
	ArtifactID string     `json:"artifact_id"`
	Name       string     `json:"name"`
	Parts      []TextPart `json:"parts"`
}

// Envelope is the turn result. The spec §4.3 schema is closed and rejects
// null, so optional members are omitted rather than set to null.
type Envelope struct {
	MediaType string     `json:"media_type"`
	State     string     `json:"a2a_state"`
	Message   *Message   `json:"message,omitempty"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

// Handle answers one turn. "needs-auth" asks for authorization out of band,
// turn 1 asks a question, and any later turn completes with one artifact. An
// error makes the runtime publish FAILED.
func Handle(_ runtime.Context, in Input) (Envelope, error) {
	if in.Turn < 1 || len(in.Parts) == 0 {
		return Envelope{}, errors.New("turns: input needs turn >= 1 and at least one part")
	}
	first := ""
	for _, p := range in.Parts {
		if p.Kind == "text" && p.Text != nil {
			first = *p.Text
			break
		}
	}
	switch {
	case strings.TrimSpace(first) == "needs-auth":
		return reply("auth-required", "Authorize the travel provider out of band, then send any message."), nil
	case in.Turn == 1:
		return reply("input-required", "How many travellers, and for which dates?"), nil
	}
	env := reply("completed", "Your itinerary is ready.")
	env.Artifacts = []Artifact{{ArtifactID: "itinerary", Name: "Itinerary",
		Parts: []TextPart{{Kind: "text", Text: fmt.Sprintf("Itinerary for task %s after %d turns.", in.TaskID, in.Turn)}}}}
	return env, nil
}

func reply(state, text string) Envelope {
	return Envelope{MediaType: MediaType, State: state, Message: &Message{Parts: []TextPart{{Kind: "text", Text: text}}}}
}

// Register registers Handle on topic and on worker.<workerID>.jobs, the direct
// subject Cordum's scheduler dispatches to once the worker heartbeats.
func Register(agent *runtime.Agent, topic, workerID string) error {
	if agent == nil || strings.TrimSpace(topic) == "" || !idRe.MatchString(workerID) {
		return errors.New("turns: Register needs an agent, a topic and a worker id matching " + idRe.String())
	}
	runtime.Register(agent, topic, Handle)
	runtime.Register(agent, "worker."+workerID+".jobs", Handle)
	return nil
}

// HeartbeatPacket builds the heartbeat that makes the worker eligible for
// dispatch in pool.
func HeartbeatPacket(workerID, pool string, maxParallel int32, capabilities []string) *agentv1.BusPacket {
	return &agentv1.BusPacket{TraceId: workerID, SenderId: workerID, ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt: timestamppb.Now(), Payload: &agentv1.BusPacket_Heartbeat{Heartbeat: &agentv1.Heartbeat{
			WorkerId: workerID, Pool: pool, Type: "cpu", MaxParallelJobs: maxParallel, Capabilities: capabilities}}}
}

// RunHeartbeats publishes build() on sys.heartbeat at once and then every
// interval until ctx ends, signing each packet with sign unless sign is nil.
// Errors are logged, never fatal. It returns after its last publish.
func RunHeartbeats(ctx context.Context, pub client.Publisher, interval time.Duration,
	build func() *agentv1.BusPacket, sign func(*agentv1.BusPacket) error) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for ctx.Err() == nil {
		if err := beat(pub, build(), sign); err != nil {
			slog.Warn("heartbeat not published", "error", err)
		}
		select {
		case <-ctx.Done():
		case <-tick.C:
		}
	}
}

func beat(pub client.Publisher, pkt *agentv1.BusPacket, sign func(*agentv1.BusPacket) error) error {
	if sign != nil {
		if err := sign(pkt); err != nil {
			return err
		}
	}
	raw, err := capsdk.MarshalDeterministic(pkt)
	if err != nil {
		return err
	}
	return pub.Publish(capsdk.SubjectHeartbeat, raw)
}
