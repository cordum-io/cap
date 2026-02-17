package client

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler processes a received BusPacket.
type Handler func(context.Context, *agentv1.BusPacket)

// Client is a CAP client.
type Client struct {
	NATS       *nats.Conn
	PublicKeys map[string]*ecdsa.PublicKey
}

// Publisher is an interface that can publish a message to a subject.
type Publisher interface {
	Publish(subject string, data []byte) error
}

// Submit publishes a JobRequest to the submit subject.
func (c *Client) Submit(ctx context.Context, req *agentv1.JobRequest, traceID, senderID string, key *ecdsa.PrivateKey) error {
	packet := &agentv1.BusPacket{
		TraceId:         traceID,
		SenderId:        senderID,
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_JobRequest{
			JobRequest: req,
		},
	}

	if key != nil {
		if err := capsdk.SignPacket(packet, key); err != nil {
			return err
		}
	}

	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	return c.NATS.Publish(capsdk.SubjectSubmit, data)
}

// Subscribe subscribes to a subject and handles incoming packets.
func (c *Client) Subscribe(subject string, handler Handler) (*nats.Subscription, error) {
	if c.NATS == nil {
		return nil, errors.New("client: NATS connection is nil")
	}
	if subject == "" {
		return nil, errors.New("client: subject is required")
	}
	if handler == nil {
		return nil, errors.New("client: handler is required")
	}

	sub, err := c.NATS.Subscribe(subject, func(msg *nats.Msg) {
		ctx := context.Background()
		var packet agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &packet); err != nil {
			log.Printf("client: %v", capsdk.NewMalformedPacketError(err.Error()))
			return
		}

		// Verify the signature
		if c.PublicKeys != nil {
			pubKey, ok := c.PublicKeys[packet.GetSenderId()]
			if !ok {
				log.Printf("client: no public key found for sender: %s", packet.GetSenderId())
				return
			}

			if len(packet.GetSignature()) == 0 {
				log.Printf("client: %v", capsdk.NewSignatureMissingError("packet from sender: "+packet.GetSenderId()))
				return
			}
			if err := capsdk.VerifyPacketSignature(&packet, pubKey); err != nil {
				log.Printf("client: %v", capsdk.NewSignatureInvalidError("sender "+packet.GetSenderId()+": "+err.Error()))
				return
			}
		}

		handler(ctx, &packet)
	})

	if err != nil {
		return nil, fmt.Errorf("subscribe: %w", err)
	}
	return sub, nil
}

// Submit is a convenience function for submitting a job without creating a Client instance.
func Submit(ctx context.Context, pub Publisher, req *agentv1.JobRequest, traceID, senderID string, key *ecdsa.PrivateKey) error {
	packet := &agentv1.BusPacket{
		TraceId:         traceID,
		SenderId:        senderID,
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload: &agentv1.BusPacket_JobRequest{
			JobRequest: req,
		},
	}

	if key != nil {
		if err := capsdk.SignPacket(packet, key); err != nil {
			return err
		}
	}

	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	return pub.Publish(capsdk.SubjectSubmit, data)
}
