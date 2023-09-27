package utils

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"testing"
	"time"
)

type EmbedStruct struct {
	Val1 string
	Val2 string
}

type TestStruct struct {
	StringAttr    string    `pubsub-attr:"string"`
	StringPtrAttr ***string `pubsub-attr:"stringptr"`
	ComplexAttr   complex64 `pubsub-attr:"complex"`
	UintAttr      uint8     `pubsub-attr:"uint"`
	UintPtrAttr   *uint8    `pubsub-attr:"uintptr"`
	Struct        EmbedStruct
	String        string
}

func TestSetAttributes(t *testing.T) {
	testStruct := TestStruct{}
	err := UnmarshalFields(
		map[string]string{
			"string":    "stringval",
			"stringptr": "stringptrval",
			"uint":      "8",
			"complex":   "(12+8i)",
			"uintptr":   "88",
		}, &testStruct, "pubsub-attr")
	Assert(t, err, IsNil)
	Assert(t, testStruct.StringAttr, Equals, "stringval")
	Assert(t, testStruct.StringPtrAttr, DeepEquals, createTriplePointer("stringptrval"))
	Assert(t, testStruct.UintAttr, Equals, uint8(8))
	Assert(t, testStruct.ComplexAttr, Equals, complex64(12+8i))
	Assert(t, testStruct.UintPtrAttr, DeepEquals, createPointer(uint8(88)))
}

type CheckType int

const (
	IsNil CheckType = iota
	Equals
	DeepEquals
	LessThanEqual
	IsTrue
)

func Assert(t *testing.T, val any, check CheckType, want ...any) {
	switch check {
	case IsNil:
		if val != nil {
			stack := debug.Stack()
			t.Fatalf("%v is not nil\n%s", val, stack)
		}
	case Equals:
		if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", want[0]) {
			stack := debug.Stack()
			t.Fatalf("%v != %v\n%s", val, want[0], stack)
		}
	case DeepEquals:
		derefVal := deref(val)
		derefWant := deref(want[0])
		if derefVal != derefWant {
			stack := debug.Stack()
			t.Fatalf("%v != %v\n%s", derefVal, derefWant, stack)
		}
	case LessThanEqual:
		fVal, err1 := strconv.ParseFloat(fmt.Sprintf("%d", val), 64)
		if err1 != nil {
			stack := debug.Stack()
			t.Fatalf("%v is not numeric\n%s", val, stack)
		}
		fWant, err2 := strconv.ParseFloat(fmt.Sprintf("%d", want[0]), 64)
		if err2 != nil {
			stack := debug.Stack()
			t.Fatalf("%v is not numeric\n%s", want[0], stack)
		}
		if fVal > fWant {
			stack := debug.Stack()
			t.Fatalf("%v > %v\n%s", val, want[0], stack)
		}
	case IsTrue:
		bVal, ok := val.(bool)
		if !ok {
			stack := debug.Stack()
			t.Fatalf("%v is not a boolv\n%s", val, stack)
		}
		if !bVal {
			stack := debug.Stack()
			t.Fatalf("%v is not True\n%s", val, stack)
		}
	}
}

func deref(val interface{}) interface{} {
	v := reflect.ValueOf(val)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return v.Interface()
}

func createPointer[T any](val T) *T {
	return &val
}

func createTriplePointer[T any](val T) ***T {
	ptr := &val
	ptr2 := &ptr
	return &ptr2
}

func TestGetAttributes(t *testing.T) {
	testStruct := &TestStruct{
		StringAttr:    "stringattrval",
		StringPtrAttr: createTriplePointer("stringptrval"),
		UintAttr:      8,
		UintPtrAttr:   createPointer(uint8(88)),
		ComplexAttr:   12 + 8i,
	}

	attrs, err := MarshalFields(testStruct, "pubsub-attr")
	Assert(t, err, IsNil)
	Assert(t, attrs["string"], Equals, "stringattrval")
	Assert(t, attrs["stringptr"], Equals, "stringptrval")
	Assert(t, attrs["uint"], Equals, "8")
	Assert(t, attrs["complex"], Equals, "(12+8i)")
	Assert(t, attrs["uintptr"], Equals, "88")
}

const maxAttempt = 100

func TestGetDelay(t *testing.T) {
	tests := map[string]*struct {
		MinRetryDelay time.Duration
		MaxRetryDelay time.Duration
		MaxRetries    int
	}{
		"limited retries": {
			MinRetryDelay: 2 * time.Second,
			MaxRetryDelay: 10 * time.Second,
			MaxRetries:    15,
		},
		"unlimited retries": {
			MinRetryDelay: 2 * time.Second,
			MaxRetryDelay: 10 * time.Second,
			MaxRetries:    -1,
		},
		"0 retries": {
			MinRetryDelay: 2 * time.Second,
			MaxRetryDelay: 10 * time.Second,
			MaxRetries:    0,
		},
		"min > max": {
			MinRetryDelay: 10 * time.Second,
			MaxRetryDelay: 2 * time.Second,
			MaxRetries:    10,
		},
		"min == max": {
			MinRetryDelay: 10 * time.Second,
			MaxRetryDelay: 10 * time.Second,
			MaxRetries:    10,
		},
		"negative delay": {
			MinRetryDelay: -10 * time.Second,
			MaxRetryDelay: 10 * time.Second,
			MaxRetries:    10,
		},
	}

	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			retry := true
			attempt := uint16(0)
			prevDelay := 0 * time.Second
			for ; retry && attempt < maxAttempt; attempt++ {
				var delay time.Duration
				retry, delay = GetDelay(policy.MaxRetries, policy.MinRetryDelay, policy.MaxRetryDelay, attempt)
				if !retry {
					continue
				}
				Assert(t, delay > prevDelay || delay == policy.MaxRetryDelay, IsTrue)
				if policy.MinRetryDelay > policy.MaxRetryDelay {
					Assert(t, delay, Equals, policy.MaxRetryDelay)
				} else if policy.MinRetryDelay < 1 && attempt == 0 {
					Assert(t, delay, Equals, 1*time.Second)
				} else if attempt == 0 {
					Assert(t, delay, Equals, policy.MinRetryDelay)
				}
				Assert(t, delay, LessThanEqual, policy.MaxRetryDelay)
				prevDelay = delay
			}
			if policy.MaxRetries == -1 {
				Assert(t, attempt, Equals, maxAttempt)
			} else {
				Assert(t, attempt, Equals, policy.MaxRetries+2) // +2 as delivery attempts are not 0 index, they start at 1
			}
		})

	}
}
