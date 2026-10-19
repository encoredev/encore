package etype_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/shared/etype"
)

type bodyErrorMarker interface{ IsRequestBodyError() }

func TestBodyErrorsAreMarked(t *testing.T) {
	var title string
	iter := jsoniter.ConfigCompatibleWithStandardLibrary.BorrowIterator([]byte(`123`))
	var parse etype.Unmarshaller
	parse.ParseJSON("Title", iter, &title)

	var read etype.Unmarshaller
	read.ReadBody(strings.NewReader(""))

	for name, err := range map[string]error{"ParseJSON": parse.Error, "ReadBody": read.Error} {
		if !errors.As(err, new(bodyErrorMarker)) {
			t.Errorf("%s: %v is not marked as a body error", name, err)
		}
	}
}

func TestParameterErrorsAreNotMarked(t *testing.T) {
	var dec etype.Unmarshaller
	etype.UnmarshalOne(&dec, strconv.Atoi, "limit", "abc", true)
	if dec.Error == nil || errors.As(dec.Error, new(bodyErrorMarker)) {
		t.Errorf("%v must be an unmarked parameter error", dec.Error)
	}
}

func TestEarlierParameterErrorIsNotMarkedByLaterBodyError(t *testing.T) {
	var dec etype.Unmarshaller
	etype.UnmarshalOne(&dec, strconv.Atoi, "limit", "abc", true)
	dec.ReadBody(strings.NewReader(""))
	if errors.As(dec.Error, new(bodyErrorMarker)) {
		t.Errorf("%v: first error wins and it is a parameter error", dec.Error)
	}
}
