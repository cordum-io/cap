package capsdk

import (
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestProductionProfileProtoTagsAreAppendOnly(t *testing.T) {
	packet := (&agentv1.BusPacket{}).ProtoReflect().Descriptor()
	assertFields(t, packet, []expectedField{
		{name: "signature_metadata", number: 5},
		{name: "identity", number: 6},
		{name: "signature", number: 14},
		{name: "auth_token", number: 18},
	})
	for tag := protoreflect.FieldNumber(23); tag <= 27; tag++ {
		if !packet.ReservedRanges().Has(tag) {
			t.Errorf("BusPacket tag %d must remain reserved", tag)
		}
	}

	jobMessages := agentv1.File_cordum_agent_v1_job_proto.Messages()
	contracts := map[protoreflect.Name][]expectedField{
		"IdentityBinding":  fields("tenant_id", "principal_id", "actor_id", "delegation_id"),
		"DispatchIdentity": fields("dispatch_id", "attempt", "assigned_worker_id"),
		"ResourceRef":      fields("resolver_id", "uri", "sha256", "media_type", "size_bytes", "expires_at", "purpose"),
		"JobRequest":       {{name: "identity", number: 18}, {name: "dispatch", number: 19}, {name: "context_ref", number: 20}},
		"Compensation":     {{name: "identity", number: 13}, {name: "dispatch", number: 14}, {name: "context_ref", number: 15}},
		"JobResult":        {{name: "dispatch", number: 10}, {name: "identity", number: 11}, {name: "result_ref", number: 12}, {name: "artifact_refs", number: 13}},
		"JobProgress":      {{name: "dispatch", number: 8}, {name: "identity", number: 9}, {name: "result_ref", number: 10}, {name: "artifact_refs", number: 11}},
		"JobCancel":        {{name: "dispatch", number: 4}, {name: "identity", number: 5}},
	}
	for name, expected := range contracts {
		t.Run(string(name), func(t *testing.T) { assertFields(t, jobMessages.ByName(name), expected) })
	}
}

func TestLegacyJobRequestWireStillParses(t *testing.T) {
	// job_id=legacy-job (field 1), topic=job.legacy (field 2), tenant_id=t1 (field 13)
	legacy := []byte{0x0a, 0x0a, 'l', 'e', 'g', 'a', 'c', 'y', '-', 'j', 'o', 'b', 0x12, 0x0a, 'j', 'o', 'b', '.', 'l', 'e', 'g', 'a', 'c', 'y', 0x6a, 0x02, 't', '1'}
	var request agentv1.JobRequest
	if err := proto.Unmarshal(legacy, &request); err != nil {
		t.Fatalf("parse legacy JobRequest: %v", err)
	}
	if request.JobId != "legacy-job" || request.Topic != "job.legacy" || request.TenantId != "t1" {
		t.Fatalf("legacy request changed: %+v", &request)
	}
}
