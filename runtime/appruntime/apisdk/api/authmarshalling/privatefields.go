package authmarshalling

import (
	"unicode"

	"github.com/json-iterator/go"
)

type unexportedFieldsSupport struct {
	jsoniter.DummyExtension
}

func (extension *unexportedFieldsSupport) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		isUnexported := unicode.IsLower(rune(binding.Field.Name()[0]))
		if isUnexported {
			binding.FromNames = []string{binding.Field.Name()}
			binding.ToNames = []string{binding.Field.Name()}
		}
	}
}
