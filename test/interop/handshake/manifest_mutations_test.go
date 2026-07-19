package main

import (
	"fmt"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func applyManifestPacketMutation(packet *agentv1.BusPacket, mutation manifestMutation, nowNanos int64) error {
	var err error
	switch mutation.Kind {
	case "set":
		err = setManifestPath(packet.ProtoReflect(), mutation)
	case "clear":
		err = clearManifestPath(packet.ProtoReflect(), mutation.Path)
	case "xor_byte":
		err = xorManifestPath(packet.ProtoReflect(), mutation)
	case "clock_offset_ns":
		err = offsetManifestTimestamp(packet.ProtoReflect(), mutation)
	case "set_time":
		err = setManifestTimestamp(packet.ProtoReflect(), mutation.Path, nowNanos)
	case "append_unknown_field":
		err = appendUnknown(packet.ProtoReflect(), mutation)
	case "append_unknown_nested_field":
		err = appendUnknownNested(packet.ProtoReflect(), mutation)
	default:
		err = fmt.Errorf("unsupported mutation kind %q", mutation.Kind)
	}
	return err
}

func resolveManifestPath(message protoreflect.Message, path string) (protoreflect.Message, protoreflect.FieldDescriptor, *string, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, nil, nil, fmt.Errorf("invalid empty mutation path")
	}
	current := message
	for index, part := range parts {
		field := current.Descriptor().Fields().ByName(protoreflect.Name(part))
		if field == nil {
			return nil, nil, nil, fmt.Errorf("unknown field %q in %q", part, path)
		}
		if index == len(parts)-1 {
			return current, field, nil, nil
		}
		if field.IsMap() {
			if index != len(parts)-2 {
				return nil, nil, nil, fmt.Errorf("map field %q is not terminal", part)
			}
			key := parts[index+1]
			return current, field, &key, nil
		}
		if field.Message() == nil {
			return nil, nil, nil, fmt.Errorf("field %q is not a message", part)
		}
		current = current.Mutable(field).Message()
	}
	return nil, nil, nil, fmt.Errorf("unresolved mutation path %q", path)
}

func setManifestPath(message protoreflect.Message, mutation manifestMutation) error {
	parent, field, mapKey, err := resolveManifestPath(message, mutation.Path)
	if err != nil {
		return err
	}
	if mapKey != nil {
		value, valueErr := mutation.boolValue()
		if valueErr != nil {
			return valueErr
		}
		parent.Mutable(field).Map().Set(protoreflect.ValueOfString(*mapKey).MapKey(), protoreflect.ValueOfBool(value))
		return nil
	}
	value, err := scalarManifestValue(field, mutation)
	if err != nil {
		return err
	}
	parent.Set(field, value)
	return nil
}

func scalarManifestValue(field protoreflect.FieldDescriptor, mutation manifestMutation) (protoreflect.Value, error) {
	switch field.Kind() {
	case protoreflect.StringKind:
		value, err := mutation.stringValue()
		return protoreflect.ValueOfString(value), err
	case protoreflect.BoolKind:
		value, err := mutation.boolValue()
		return protoreflect.ValueOfBool(value), err
	case protoreflect.Int32Kind:
		value, err := mutation.intValue()
		return protoreflect.ValueOfInt32(int32(value)), err
	case protoreflect.Int64Kind:
		value, err := mutation.intValue()
		return protoreflect.ValueOfInt64(value), err
	case protoreflect.Uint32Kind:
		value, err := mutation.intValue()
		return protoreflect.ValueOfUint32(uint32(value)), err
	case protoreflect.Uint64Kind:
		value, err := mutation.intValue()
		return protoreflect.ValueOfUint64(uint64(value)), err
	default:
		return protoreflect.Value{}, fmt.Errorf("field %s has unsupported set kind %s", field.FullName(), field.Kind())
	}
}

func clearManifestPath(message protoreflect.Message, path string) error {
	parent, field, mapKey, err := resolveManifestPath(message, path)
	if err != nil {
		return err
	}
	if mapKey != nil {
		parent.Mutable(field).Map().Clear(protoreflect.ValueOfString(*mapKey).MapKey())
		return nil
	}
	parent.Clear(field)
	return nil
}

func xorManifestPath(message protoreflect.Message, mutation manifestMutation) error {
	parent, field, _, err := resolveManifestPath(message, mutation.Path)
	if err != nil {
		return err
	}
	mask, err := mutation.intValue()
	if err != nil {
		return err
	}
	switch field.Kind() {
	case protoreflect.StringKind:
		value := []byte(parent.Get(field).String())
		if len(value) == 0 {
			return fmt.Errorf("cannot xor empty field %s", field.FullName())
		}
		value[0] ^= byte(mask)
		parent.Set(field, protoreflect.ValueOfString(string(value)))
	case protoreflect.BytesKind:
		value := append([]byte(nil), parent.Get(field).Bytes()...)
		if len(value) == 0 {
			return fmt.Errorf("cannot xor empty field %s", field.FullName())
		}
		value[0] ^= byte(mask)
		parent.Set(field, protoreflect.ValueOfBytes(value))
	default:
		return fmt.Errorf("cannot xor field kind %s", field.Kind())
	}
	return nil
}

func offsetManifestTimestamp(message protoreflect.Message, mutation manifestMutation) error {
	parent, field, _, err := resolveManifestPath(message, mutation.Path)
	if err != nil {
		return err
	}
	offset, err := mutation.intValue()
	if err != nil {
		return err
	}
	timestamp, ok := parent.Get(field).Message().Interface().(*timestamppb.Timestamp)
	if !ok {
		return fmt.Errorf("field %s is not a timestamp", field.FullName())
	}
	parent.Set(field, protoreflect.ValueOfMessage(timestamppb.New(timestamp.AsTime().Add(time.Duration(offset))).ProtoReflect()))
	return nil
}

func setManifestTimestamp(message protoreflect.Message, path string, nanos int64) error {
	parent, field, _, err := resolveManifestPath(message, path)
	if err != nil {
		return err
	}
	value := timestamppb.New(time.Unix(0, nanos).UTC())
	parent.Set(field, protoreflect.ValueOfMessage(value.ProtoReflect()))
	return nil
}

func appendUnknown(message protoreflect.Message, mutation manifestMutation) error {
	number, err := mutation.intValue()
	if err != nil {
		return err
	}
	unknown := protowire.AppendTag(message.GetUnknown(), protowire.Number(number), protowire.BytesType)
	message.SetUnknown(protowire.AppendBytes(unknown, []byte("unknown")))
	return nil
}

func appendUnknownNested(message protoreflect.Message, mutation manifestMutation) error {
	oneof := message.Descriptor().Oneofs().ByName("payload")
	if oneof == nil {
		return fmt.Errorf("packet payload oneof unavailable")
	}
	field := message.WhichOneof(oneof)
	if field == nil || field.Message() == nil {
		return fmt.Errorf("packet payload unavailable")
	}
	return appendUnknown(message.Mutable(field).Message(), mutation)
}
