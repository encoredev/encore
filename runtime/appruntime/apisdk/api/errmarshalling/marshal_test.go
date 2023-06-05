package errmarshalling_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modern-go/reflect2"

	"encore.dev/appruntime/apisdk/api/errmarshalling"
	"encore.dev/beta/errs"
	"encore.dev/storage/cache"
	"encore.dev/storage/sqldb"
	"encore.dev/storage/sqldb/sqlerr"
)

type SomeRandomType struct {
	Foo string
	Bar bool
	Err error
}

func TestMarshal(t *testing.T) {
	params := []struct {
		name string
		err  error
	}{
		{"basic", errors.New("blah")},
		{"joined errors", errors.Join(errors.New("error a"), errors.New("error b"))},
		{"single wrapped error", fmt.Errorf("outer error: %w", errors.New("inner issue"))},
		{"3 deep single wrapped error", fmt.Errorf("outer error: %w", fmt.Errorf("inner issue: %w", errors.New("inner inner issue")))},
		{"multiple wrapped errors", fmt.Errorf("outer error: %w", errors.Join(errors.New("inner issue"), errors.New("inner issue 2")))},
		{"encore errs", errs.B().Code(errs.OutOfRange).Meta("foo", "bar", "x", 123, "y", false).Msg("out of range in foo").Err()},
		{"encore err wrapped in Go error", fmt.Errorf("outer: %w", errs.B().Code(errs.OutOfRange).Msg("out of range in foo").Err())},
		{"sqldb error", &sqldb.Error{
			Code:           sqlerr.NotNullViolation,
			Severity:       sqlerr.SeverityError,
			DatabaseCode:   "PG2341",
			Message:        "Not null happened matey",
			SchemaName:     "your_database",
			TableName:      "in_this_table",
			ColumnName:     "on_this_column",
			DataTypeName:   "string",
			ConstraintName: "not_null",
		}},
		{"cache operror", &cache.OpError{
			Operation: "get",
			RawKey:    "foo",
			Err:       errors.New("some error"),
		}},
	}

	for _, p := range params {
		p := p
		t.Run(p.name, func(t *testing.T) {
			_ = roundTrip(t, p.err)
		})
	}

	t.Run("encore err with underlying", func(t *testing.T) {
		inner := errs.B().Code(errs.OutOfRange).Msg("out of range in foo").Err()
		errIn := errs.B().Code(errs.Internal).
			Cause(inner).
			Meta("hello", 1, "goodbye", 2).
			Msg("blah10").
			Err()
		errOut := roundTrip(t, errIn)

		fmt.Println("Error In: ", errIn)
		fmt.Println("Error Out:", errOut)
	})
}

func roundTrip(t *testing.T, err error) error {
	bytes := errmarshalling.Marshal(err)
	fmt.Println(string(bytes))

	unmarshalled, unmarshallingErr := errmarshalling.Unmarshal(bytes)
	if unmarshallingErr != nil {
		t.Fatalf(
			"Failed to unmarshal error\n\nGot: %v\n\nErr: %v", err, unmarshallingErr)
		return nil
	}

	if unmarshalled == nil {
		t.Fatalf("Expected error %v, got nil", err)
	}

	if unmarshalled.Error() != err.Error() {
		t.Errorf("Expected error %v, got %v", err, unmarshalled)
	}

	originalType := reflect2.TypeOf(err)
	unmarshalledType := reflect2.TypeOf(unmarshalled)

	if originalType.String() != unmarshalledType.String() &&
		originalType.String() != "*errors.joinError" &&
		originalType.String() != "*fmt.wrapError" {
		fmt.Println("Original type:", originalType.Type1().PkgPath())
		t.Errorf("Expected error type %v, got %v", originalType, unmarshalledType)
	}

	return unmarshalled
}

func TestMarshalWithCustomData(t *testing.T) {
	data := SomeRandomType{
		Foo: "hello",
		Bar: true,
		Err: errs.B().Code(errs.OutOfRange).Meta("a", "b").Msg("out of range in foo").Err(),
	}

	json := errmarshalling.JsonAPI()

	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	data2 := SomeRandomType{}
	if err := json.Unmarshal(bytes, &data2); err != nil {
		t.Fatalf("unable to unmarshal: %v", err)
	}

	unmarshalledMeta := errs.Meta(data2.Err)
	if len(unmarshalledMeta) == 0 {
		t.Fatalf("expected metadata to be unmarshalled")
	}
	if unmarshalledMeta["a"] != "b" {
		t.Fatalf("unmarshalled metadata is incorrect: %v", unmarshalledMeta)
	}
}
