package a2abridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/a2aproject/a2a-go/v2/a2a"
	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EnvelopeMediaType identifies the strict turn-result grammar in spec §4.
const EnvelopeMediaType = "application/vnd.cordum.cap.a2a-turn-result.v1+json"
const maxEnvelopeBytes = 262144

// ResultReader reads result storage. Implementations must return when ctx ends,
// fail without reading a value larger than maxBytes, and support concurrent use.
type ResultReader interface {
	ReadResult(ctx context.Context, key string, maxBytes int) ([]byte, error)
}

// ProductionTrustFunc returns the CAP-PRODUCTION trust store for one audience
// (the subject), tenant (the task's, never the packet's) and sender, or false
// when that sender is not trusted there. It must be safe for concurrent use.
type ProductionTrustFunc func(audience, tenant, sender string) (capsdk.ProductionTrustStore, bool)

// Subscriber subscribes a handler to a NATS subject. *nats.Conn satisfies it.
type Subscriber interface {
	Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error)
}

var _ Subscriber = (*nats.Conn)(nil)

// egressSubjects carry the packets that move a turn (spec §2.1): worker
// results, scheduler-side outcomes such as DENIED, and progress hints.
var egressSubjects = [...]string{capsdk.SubjectResult, capsdk.SubjectDLQ, capsdk.SubjectProgress}

// publishCancel mirrors client.submit's signed deterministic packet envelope.
// The caller's identity and the store, never the request, select principal/job.
func (i *Ingress) publishCancel(id Identity, taskID, job string) error {
	if i.signer == nil && !i.unsigned {
		return errors.New("a2abridge: cancel signing key required")
	}
	pkt := &agentv1.BusPacket{TraceId: taskID, SenderId: i.sender, CreatedAt: timestamppb.New(i.now()),
		ProtocolVersion: capsdk.DefaultProtocolVersion, Payload: &agentv1.BusPacket_JobCancel{
			JobCancel: &agentv1.JobCancel{JobId: job, Reason: "a2a CancelTask", RequestedBy: id.Principal}}}
	if i.signer != nil {
		if err := capsdk.SignPacket(pkt, i.signer); err != nil {
			return err
		}
	}
	raw, err := capsdk.MarshalDeterministic(pkt)
	if err != nil {
		return err
	}
	return i.pub.Publish(capsdk.SubjectCancel, raw)
}

// Start subscribes to results, DLQ outcomes and progress, and applies each
// verified packet to its task. It fails closed without a Subscriber, Results or a
// verification mode (ProductionTrust, ResultKeys or AllowUnsignedDev). The
// returned stop unsubscribes every subscription.
func (i *Ingress) Start(sub Subscriber) (stop func() error, err error) {
	if sub == nil {
		return nil, errors.New("a2abridge: Start needs a Subscriber")
	}
	if i.results == nil {
		return nil, errors.New("a2abridge: Start needs Results")
	}
	if i.trust == nil && len(i.resultKeys) == 0 && !i.unsigned {
		return nil, errors.New("a2abridge: Start needs ProductionTrust, ResultKeys or AllowUnsignedDev")
	}
	subs := make([]*nats.Subscription, 0, len(egressSubjects))
	stop = func() error {
		errs := make([]error, 0, len(subs))
		for _, s := range subs {
			errs = append(errs, s.Unsubscribe())
		}
		return errors.Join(errs...)
	}
	for _, subject := range egressSubjects {
		s, err := sub.Subscribe(subject, func(m *nats.Msg) { i.deliver(m.Subject, m.Data) })
		if err != nil {
			return nil, errors.Join(fmt.Errorf("a2abridge: subscribe %s: %w", subject, err), stop())
		}
		subs = append(subs, s)
	}
	i.started.Store(true) // only now can a turn settle, so SendMessage may wait
	return stop, nil
}

// deliver applies one packet that arrived on subject. A packet that fails
// verification, is on the wrong subject or names no current job is dropped
// silently and changes nothing.
func (i *Ingress) deliver(subject string, raw []byte) {
	pkt, ok := i.verify(subject, raw)
	if !ok {
		return
	}
	now := i.now().UTC()
	switch p := pkt.GetPayload().(type) {
	case *agentv1.BusPacket_JobResult:
		if subject == capsdk.SubjectResult || subject == capsdk.SubjectDLQ {
			job := p.JobResult.GetJobId()
			if p.JobResult.GetStatus() == agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
				if turn, ok := i.store.reserveResult(job); ok {
					out := i.readEnvelope(job, p.JobResult)
					i.store.commitResult(job, turn, out, i.now().UTC())
				}
			} else {
				i.store.applyResult(job, p.JobResult.GetStatus(), now)
			}
		}
	case *agentv1.BusPacket_JobProgress:
		if subject == capsdk.SubjectProgress {
			i.store.applyProgress(p.JobProgress.GetJobId(), p.JobProgress.GetStatus(), now)
		}
	}
}

func (i *Ingress) readEnvelope(job string, res *agentv1.JobResult) *turnOutcome {
	if i.results == nil || res.GetResultPtr() != "redis://res:"+job {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := i.results.ReadResult(ctx, "res:"+job, maxEnvelopeBytes)
	if err != nil || ctx.Err() != nil || len(raw) > maxEnvelopeBytes {
		return nil
	}
	out, err := parseEnvelope(raw)
	if err != nil {
		return nil
	}
	return out
}

var errEnvelope = errors.New("a2abridge: invalid turn-result envelope")

// strictDecode requires exact schema keys, rejecting duplicates and fold aliases.
// The required data member may itself be any JSON value (spec §4.3).
func strictDecode(raw []byte, dst interface{}, allowed ...string) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		return errEnvelope
	}
	if _, err := d.Token(); err != io.EOF {
		return errEnvelope
	}
	f := json.NewDecoder(bytes.NewReader(raw))
	if token, err := f.Token(); err != nil || token != json.Delim('{') {
		return errEnvelope
	}
	seen := map[string]bool{}
	for f.More() {
		token, err := f.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] || !slices.Contains(allowed, key) {
			return errEnvelope
		}
		seen[key] = true
		var value json.RawMessage
		if f.Decode(&value) != nil {
			return errEnvelope
		}
		if key != "data" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errEnvelope
		}
	}
	return nil
}

func parseEnvelope(raw []byte) (*turnOutcome, error) {
	if len(raw) > maxEnvelopeBytes || !utf8.Valid(raw) {
		return nil, errEnvelope
	}
	var env struct {
		MediaType string            `json:"media_type"`
		State     string            `json:"a2a_state"`
		Message   json.RawMessage   `json:"message"`
		Artifacts []json.RawMessage `json:"artifacts"`
	}
	if strictDecode(raw, &env, "media_type", "a2a_state", "message", "artifacts") != nil || env.MediaType != EnvelopeMediaType {
		return nil, errEnvelope
	}
	out := &turnOutcome{}
	switch env.State {
	case "completed":
		out.state = a2a.TaskStateCompleted
	case "input-required":
		out.state = a2a.TaskStateInputRequired
	case "auth-required":
		out.state = a2a.TaskStateAuthRequired
	default:
		return nil, errEnvelope
	}
	textOnly := out.state == a2a.TaskStateAuthRequired
	if len(env.Artifacts) > 8 || textOnly && len(env.Artifacts) != 0 {
		return nil, errEnvelope
	}
	if env.Message != nil {
		msg, err := parseMessage(env.Message, textOnly)
		if err != nil {
			return nil, errEnvelope
		}
		out.message = msg
	} else if out.state != a2a.TaskStateCompleted {
		return nil, errEnvelope
	}
	for _, rawArtifact := range env.Artifacts {
		artifact, err := parseArtifact(rawArtifact)
		if err != nil {
			return nil, errEnvelope
		}
		out.artifacts = append(out.artifacts, artifact)
	}
	return out, nil
}

func parseMessage(raw []byte, textOnly bool) (*a2a.Message, error) {
	var msg struct {
		Parts []json.RawMessage `json:"parts"`
	}
	if strictDecode(raw, &msg, "parts") != nil {
		return nil, errEnvelope
	}
	parts, err := parseParts(msg.Parts, textOnly)
	if err != nil {
		return nil, errEnvelope
	}
	return a2a.NewMessage(a2a.MessageRoleAgent, parts...), nil
}

func parseArtifact(raw []byte) (*a2a.Artifact, error) {
	var a struct {
		ID    string            `json:"artifact_id"`
		Name  string            `json:"name"`
		Parts []json.RawMessage `json:"parts"`
	}
	if strictDecode(raw, &a, "artifact_id", "name", "parts") != nil || !validID(a.ID) || utf8.RuneCountInString(a.Name) > 256 {
		return nil, errEnvelope
	}
	parts, err := parseParts(a.Parts, false)
	if err != nil {
		return nil, errEnvelope
	}
	return &a2a.Artifact{ID: a2a.ArtifactID(a.ID), Name: a.Name, Parts: parts}, nil
}

func parseParts(raw []json.RawMessage, textOnly bool) (a2a.ContentParts, error) {
	if len(raw) < 1 || len(raw) > maxParts {
		return nil, errEnvelope
	}
	parts := make(a2a.ContentParts, 0, len(raw))
	for _, item := range raw {
		part, err := parsePart(item, textOnly)
		if err != nil {
			return nil, errEnvelope
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func parsePart(raw []byte, textOnly bool) (*a2a.Part, error) {
	var p struct {
		Kind      string          `json:"kind"`
		Text      *string         `json:"text"`
		Data      json.RawMessage `json:"data"`
		MediaType *string         `json:"media_type"`
	}
	if strictDecode(raw, &p, "kind", "text", "data", "media_type") != nil {
		return nil, errEnvelope
	}
	var part *a2a.Part
	switch p.Kind {
	case "text":
		if p.Text == nil || p.Data != nil || len(*p.Text) > maxTextBytes {
			return nil, errEnvelope
		}
		part = a2a.NewTextPart(*p.Text)
	case "data":
		if textOnly || p.Text != nil || p.Data == nil {
			return nil, errEnvelope
		}
		var compact bytes.Buffer
		if json.Compact(&compact, p.Data) != nil || compact.Len() > maxDataBytes || jsonDepth(p.Data) > maxDataDepth {
			return nil, errEnvelope
		}
		var value interface{}
		d := json.NewDecoder(bytes.NewReader(p.Data))
		d.UseNumber()
		if d.Decode(&value) != nil {
			return nil, errEnvelope
		}
		part = a2a.NewDataPart(value)
	default:
		return nil, errEnvelope
	}
	if p.MediaType != nil {
		if len(*p.MediaType) > maxMediaTypeLen || !mediaTypeRe.MatchString(*p.MediaType) {
			return nil, errEnvelope
		}
		part.MediaType = *p.MediaType
	}
	return part, nil
}

// verify returns the packet only when it passes the configured mode and
// capsdk.ValidateBusPacket (a result's worker_id must equal its sender_id).
// Identity comes only from the configured keys; no trace header is read.
func (i *Ingress) verify(subject string, raw []byte) (*agentv1.BusPacket, bool) {
	if len(raw) == 0 || len(raw) > capsdk.DefaultProductionMaxRawBytes {
		return nil, false
	}
	var pkt *agentv1.BusPacket
	switch {
	case i.trust != nil:
		pkt = i.verifyProduction(subject, raw)
	case len(i.resultKeys) > 0:
		pkt = i.verifyLegacy(raw)
	case i.unsigned:
		pkt = &agentv1.BusPacket{}
		if proto.Unmarshal(raw, pkt) != nil {
			pkt = nil
		}
	}
	if pkt == nil || capsdk.ValidateBusPacket(pkt) != nil {
		return nil, false
	}
	return pkt, true
}

// verifyProduction checks a CAP-PRODUCTION packet against the trust store of
// the tenant that owns the named job's task.
func (i *Ingress) verifyProduction(subject string, raw []byte) *agentv1.BusPacket {
	var peek agentv1.BusPacket // UNVERIFIED: it only names the job and sender to check
	if proto.Unmarshal(raw, &peek) != nil {
		return nil
	}
	job := peek.GetJobResult().GetJobId()
	if job == "" {
		job = peek.GetJobProgress().GetJobId()
	}
	tenant, ok := i.store.owner(job)
	if !ok || tenant != peek.GetIdentity().GetTenantId() {
		return nil
	}
	trust, ok := i.trust(subject, tenant, peek.GetSenderId())
	if !ok {
		return nil
	}
	pkt, err := capsdk.VerifyProductionPacket(raw, trust)
	if err != nil {
		return nil
	}
	return pkt
}

// verifyLegacy requires a signature by the sender's key in ResultKeys.
func (i *Ingress) verifyLegacy(raw []byte) *agentv1.BusPacket {
	pkt := &agentv1.BusPacket{}
	if proto.Unmarshal(raw, pkt) != nil {
		return nil
	}
	key := i.resultKeys[pkt.GetSenderId()]
	if key == nil || len(pkt.GetSignature()) == 0 || capsdk.VerifyPacketSignature(pkt, key) != nil {
		return nil
	}
	return pkt
}
