package capsdk

import (
	"os"
	"path/filepath"
	"testing"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/proto"
)

func TestWorkerTrustValidatorsAcceptPublishedHandshakeFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "spec", "conformance", "fixtures", "handshake")
	for _, name := range []string{"challenge_request.bin", "challenge.bin", "authenticate.bin", "result.bin"} {
		t.Run(name, func(t *testing.T) {
			packet := readWorkerTrustFixture(t, root, name)
			if err := ValidateWorkerTrustPacket(packet); err != nil {
				t.Fatalf("ValidateWorkerTrustPacket(%s): %v", name, err)
			}
		})
	}
}

func TestWorkerTrustValidatorRejectsUnsupportedFixtureVersion(t *testing.T) {
	root := filepath.Join("..", "..", "spec", "conformance", "fixtures", "handshake")
	packet := readWorkerTrustFixture(t, root, "challenge_request.bin")
	packet.ProtocolVersion = 2
	if err := ValidateWorkerTrustPacket(packet); err == nil {
		t.Fatal("ValidateWorkerTrustPacket accepted protocol_version=2")
	}
}

func readWorkerTrustFixture(t *testing.T, root, name string) *agentv1.BusPacket {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	packet := &agentv1.BusPacket{}
	if err := proto.Unmarshal(data, packet); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return packet
}
