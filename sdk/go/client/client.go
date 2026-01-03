package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"

	"github.com/coretexos/cap/v2/sdk/go"
	agentv1 "github.com/coretexos/cap/v2/coretex/agent/v1"
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
		// Marshal the packet without the signature for signing
		unsignedData, err := proto.Marshal(packet)
		if err != nil {
			return fmt.Errorf("marshal unsigned packet: %w", err)
		}

		// Sign the data
		hash := sha256.Sum256(unsignedData)
		signature, err := ecdsa.SignASN1(rand.Reader, key, hash[:])
		if err != nil {
			return fmt.Errorf("sign packet: %w", err)
		}
		packet.Signature = signature
	}

	data, err := proto.Marshal(packet)
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
			log.Printf("client: could not decode packet: %v", err)
			return
		}

		// Verify the signature
		if c.PublicKeys != nil {
			pubKey, ok := c.PublicKeys[packet.GetSenderId()]
			if !ok {
				log.Printf("client: no public key found for sender: %s", packet.GetSenderId())
				return
			}

			signature := packet.Signature
			packet.Signature = nil
			unsignedData, err := proto.Marshal(&packet)
			if err != nil {
				log.Printf("client: could not marshal unsigned packet for verification: %v", err)
				return
			}
			hash := sha256.Sum256(unsignedData)
			if !ecdsa.VerifyASN1(pubKey, hash[:], signature) {
				log.Printf("client: invalid signature for packet from sender: %s", packet.GetSenderId())
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
		// Marshal the packet without the signature for signing
		unsignedData, err := proto.Marshal(packet)
		if err != nil {
			return fmt.Errorf("marshal unsigned packet: %w", err)
		}

		// Sign the data
		hash := sha256.Sum256(unsignedData)
		signature, err := ecdsa.SignASN1(rand.Reader, key, hash[:])
		if err != nil {
			return fmt.Errorf("sign packet: %w", err)
		}
		packet.Signature = signature
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	return pub.Publish(capsdk.SubjectSubmit, data)
}
