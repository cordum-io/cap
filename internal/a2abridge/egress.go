package a2abridge

import (
	"errors"
	"fmt"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

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

// Start subscribes to results, DLQ outcomes and progress, and applies each
// verified packet to its task. It fails closed without a Subscriber or a
// verification mode (ProductionTrust, ResultKeys or AllowUnsignedDev). The
// returned stop unsubscribes every subscription.
func (i *Ingress) Start(sub Subscriber) (stop func() error, err error) {
	if sub == nil {
		return nil, errors.New("a2abridge: Start needs a Subscriber")
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
			i.store.applyResult(p.JobResult.GetJobId(), p.JobResult.GetStatus(), now)
		}
	case *agentv1.BusPacket_JobProgress:
		if subject == capsdk.SubjectProgress {
			i.store.applyProgress(p.JobProgress.GetJobId(), p.JobProgress.GetStatus(), now)
		}
	}
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
