package capsdk

import "google.golang.org/protobuf/encoding/protowire"

func validateUnsignedProductionWire(unsigned []byte) error {
	// Reuse the verifier grammar with a placeholder signature so producers
	// cannot emit bytes that the current production profile rejects.
	probe := appendSignatureField(unsigned, []byte{1})
	_, _, err := extractSignatureField(probe)
	return err
}

func extractSignatureField(raw []byte) ([]byte, []byte, error) {
	if len(raw) > DefaultProductionMaxRawBytes {
		return nil, nil, ErrMalformedProductionWire
	}
	unsigned := make([]byte, 0, len(raw))
	var signature []byte
	seen := make(map[protowire.Number]struct{}, 18)
	payloadSeen := false
	for offset := 0; offset < len(raw); {
		fieldStart := offset
		number, wireType, n, canonical := consumeCanonicalTag(raw[offset:])
		if n < 0 || !canonical {
			return nil, nil, ErrMalformedProductionWire
		}
		if err := validateProductionOuterField(seen, &payloadSeen, number, wireType); err != nil {
			return nil, nil, err
		}
		offset += n
		valueLength, canonical := consumeCanonicalFieldValue(raw[offset:], number, wireType)
		if valueLength < 0 || !canonical || offset+valueLength > len(raw) {
			return nil, nil, ErrMalformedProductionWire
		}
		fieldEnd := offset + valueLength
		if number == 14 {
			value, consumed := protowire.ConsumeBytes(raw[offset:fieldEnd])
			if consumed != valueLength || len(value) == 0 {
				return nil, nil, ErrMalformedProductionWire
			}
			signature = append([]byte(nil), value...)
		} else {
			unsigned = append(unsigned, raw[fieldStart:fieldEnd]...)
		}
		offset = fieldEnd
	}
	if signature == nil {
		return nil, nil, ErrMissingSignature
	}
	return unsigned, signature, nil
}

func validateProductionOuterField(
	seen map[protowire.Number]struct{}, payloadSeen *bool,
	number protowire.Number, wireType protowire.Type,
) error {
	// CAP-PRODUCTION v1 uses a closed outer grammar. New envelope fields need
	// an explicit profile-version update before old verifiers may accept them.
	expectedType, known := productionBusPacketWireType(number)
	if !known || wireType != expectedType {
		return ErrMalformedProductionWire
	}
	if _, duplicate := seen[number]; duplicate {
		if number == 14 {
			return ErrDuplicateSignatureField
		}
		return ErrMalformedProductionWire
	}
	seen[number] = struct{}{}
	isPayload := productionPayloadField(number)
	if isPayload && *payloadSeen {
		return ErrMalformedProductionWire
	}
	if isPayload {
		*payloadSeen = true
	}
	return nil
}

func productionBusPacketWireType(number protowire.Number) (protowire.Type, bool) {
	switch number {
	case 4:
		return protowire.VarintType, true
	case 1, 2, 3, 5, 6, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22:
		return protowire.BytesType, true
	default:
		return 0, false
	}
}

func productionPayloadField(number protowire.Number) bool {
	switch number {
	case 10, 11, 12, 13, 15, 16, 17, 19, 20, 21, 22:
		return true
	default:
		return false
	}
}

func consumeCanonicalTag(raw []byte) (protowire.Number, protowire.Type, int, bool) {
	number, wireType, size := protowire.ConsumeTag(raw)
	if size < 0 {
		return 0, 0, size, false
	}
	canonical := protowire.AppendTag(nil, number, wireType)
	return number, wireType, size, len(canonical) == size && string(canonical) == string(raw[:size])
}

func consumeCanonicalFieldValue(
	raw []byte, number protowire.Number, wireType protowire.Type,
) (int, bool) {
	switch wireType {
	case protowire.VarintType:
		value, size := protowire.ConsumeVarint(raw)
		canonical := canonicalVarint(raw, size)
		return size, canonical && (number != 4 || value == uint64(DefaultProtocolVersion))
	case protowire.Fixed32Type:
		return 4, len(raw) >= 4
	case protowire.Fixed64Type:
		return 8, len(raw) >= 8
	case protowire.BytesType:
		length, prefix := protowire.ConsumeVarint(raw)
		if !canonicalVarint(raw, prefix) || length > uint64(len(raw)-prefix) {
			return -1, false
		}
		return prefix + int(length), true
	default:
		return -1, false
	}
}

func canonicalVarint(raw []byte, size int) bool {
	if size <= 0 || size > len(raw) {
		return false
	}
	value, _ := protowire.ConsumeVarint(raw[:size])
	canonical := protowire.AppendVarint(nil, value)
	return len(canonical) == size && string(canonical) == string(raw[:size])
}
