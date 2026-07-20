package capsdk

import "google.golang.org/protobuf/reflect/protoreflect"

func validateNoProductionUnknowns(message protoreflect.Message) error {
	if !message.IsValid() || len(message.GetUnknown()) != 0 {
		return ErrMalformedProductionWire
	}
	var invalid error
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		invalid = validateProductionFieldMessages(field, value)
		return invalid == nil
	})
	return invalid
}

func validateProductionFieldMessages(
	field protoreflect.FieldDescriptor,
	value protoreflect.Value,
) error {
	if field.IsMap() {
		return validateProductionMapMessages(field, value.Map())
	}
	if field.IsList() {
		list := value.List()
		for index := 0; index < list.Len(); index++ {
			if err := validateProductionMessageValue(field, list.Get(index)); err != nil {
				return err
			}
		}
		return nil
	}
	return validateProductionMessageValue(field, value)
}

func validateProductionMapMessages(
	field protoreflect.FieldDescriptor,
	values protoreflect.Map,
) error {
	if field.MapValue().Kind() != protoreflect.MessageKind {
		return nil
	}
	var invalid error
	values.Range(func(_ protoreflect.MapKey, value protoreflect.Value) bool {
		invalid = validateNoProductionUnknowns(value.Message())
		return invalid == nil
	})
	return invalid
}

func validateProductionMessageValue(
	field protoreflect.FieldDescriptor,
	value protoreflect.Value,
) error {
	if field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.GroupKind {
		return nil
	}
	return validateNoProductionUnknowns(value.Message())
}
