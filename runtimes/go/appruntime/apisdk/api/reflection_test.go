package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	qt "github.com/frankban/quicktest"
)

func Test_createReflectionCaller(t *testing.T) {
	c := qt.New(t)
	c.Parallel()

	c.Run("test parameter types are type checked", func(c *qt.C) {
		c.Run("missing context param", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, Void](reflect.ValueOf(func() {}))
			c.Assert(err, qt.ErrorMatches, ".*expected 1 parameters \\(context.Context\\), got 0 parameters")
		})

		c.Run("context param is wrong type", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, Void](reflect.ValueOf(func(int) {}))
			c.Assert(err, qt.ErrorMatches, ".*expected parameter 1 to be context.Context, got int")
		})

		c.Run("missing parameter type", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreExampleParamsRequest, Void](reflect.ValueOf(func(context.Context) {}))
			c.Assert(err, qt.ErrorMatches, ".*expected 2 parameters \\(context.Context, \\*api.ExampleParams\\), got 1 parameters")
		})

		c.Run("parameter type is wrong", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreExampleParamsRequest, Void](reflect.ValueOf(func(context.Context, int) {}))
			c.Assert(err, qt.ErrorMatches, ".*expected parameter 2 to be \\*api.ExampleParams, got int")
		})

		c.Run("path params are type checks", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreExampleWithPathParams, Void](reflect.ValueOf(func(context.Context, *ExampleParams, string, int, []string) {}))
			c.Assert(err, qt.ErrorMatches, ".*expected parameter 2 to be string, got \\*api.ExampleParams")
		})
	})

	c.Run("test response types are type checked", func(c *qt.C) {
		c.Run("void api doesn't return error", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, Void](reflect.ValueOf(func(context.Context) {}))
			c.Assert(err, qt.ErrorMatches, ".*expected one return value \\(error\\), got 0 return values")
		})

		c.Run("void api returns too many values", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, Void](reflect.ValueOf(func(context.Context) (error, Void) { return nil, Void{} }))
			c.Assert(err, qt.ErrorMatches, ".*expected one return value \\(error\\), got 2 return values")
		})

		c.Run("api returns nothing", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, *ExampleResponse](reflect.ValueOf(func(context.Context) {}))
			c.Assert(err, qt.ErrorMatches, ".*expected two return values \\(\\*api.ExampleResponse, error\\), got 0 return values")
		})

		c.Run("api response type is wrong", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, *ExampleResponse](reflect.ValueOf(func(context.Context) (int, error) { return 0, nil }))
			c.Assert(err, qt.ErrorMatches, ".*expected first return value to be \\*api.ExampleResponse, got int")
		})

		c.Run("api error type is wrong", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, *ExampleResponse](reflect.ValueOf(func(context.Context) (*ExampleResponse, int) { return nil, 0 }))
			c.Assert(err, qt.ErrorMatches, ".*expected second return value to be an error, got int")
		})

		c.Run("api returns too many values", func(c *qt.C) {
			c.Parallel()
			_, err := createReflectionCaller[*EncoreEmptyReq, *ExampleResponse](reflect.ValueOf(func(context.Context) (*ExampleResponse, error, int) { return nil, nil, 0 }))
			c.Assert(err, qt.ErrorMatches, ".*expected two return values \\(\\*api.ExampleResponse, error\\), got 3 return values")
		})
	})

	c.Run("test calling the returned functions", func(c *qt.C) {
		c.Run("basic api only returning an error", func(c *qt.C) {
			c.Parallel()
			called := false
			shouldError := false

			method, err := createReflectionCaller[*EncoreEmptyReq, Void](reflect.ValueOf(func(ctx context.Context) error {
				if shouldError {
					return errors.New("test error")
				}
				called = true
				return nil
			}))
			c.Assert(err, qt.IsNil, qt.Commentf("unable to create the reflection caller"))

			// Test the method works and actually gets called
			resp, err := method(context.Background(), nil)
			c.Assert(err, qt.IsNil, qt.Commentf("api should return no error"))
			c.Assert(resp, qt.Equals, Void{}, qt.Commentf("api should return Void{}"))
			c.Assert(called, qt.Equals, true, qt.Commentf("api should have been called"))

			// Test the error is returned if the API errors
			shouldError = true
			resp, err = method(context.Background(), nil)
			c.Assert(err, qt.ErrorMatches, "test error", qt.Commentf("api should return an error now"))
		})

		c.Run("api that takes parameters but returns nothing", func(c *qt.C) {
			c.Parallel()
			calledWith := "<not called>"

			method, err := createReflectionCaller[*EncoreExampleParamsRequest, Void](reflect.ValueOf(func(ctx context.Context, params *ExampleParams) error {
				if params.Param2 == "error" {
					return errors.New(params.Param1)
				}

				calledWith = params.Param1
				return nil
			}))
			c.Assert(err, qt.IsNil, qt.Commentf("unable to create the reflection caller"))

			// Test the method works and actually gets called
			resp, err := method(context.Background(), &EncoreExampleParamsRequest{
				Payload: &ExampleParams{
					Param1: "this value",
					Param2: "ignored",
				},
			})
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.Equals, Void{}, qt.Commentf("api should return Void{}"))
			c.Assert(calledWith, qt.Equals, "this value", qt.Commentf("api should have been called with the correct value"))

			// Test the error is returned if the API errors
			resp, err = method(context.Background(), &EncoreExampleParamsRequest{
				Payload: &ExampleParams{
					Param1: "my amazing error message",
					Param2: "error",
				},
			})
			c.Assert(err, qt.ErrorMatches, "my amazing error message", qt.Commentf("api should return an error now"))
		})

		c.Run("api that takes path parameters", func(c *qt.C) {
			c.Parallel()

			calledWith := "<not called>"
			method, err := createReflectionCaller[*EncoreExampleWithPathParams, Void](reflect.ValueOf(func(ctx context.Context, name string, age int, tags []string, params *ExampleParams) error {
				calledWith = fmt.Sprintf("%s - %s=%d %v", params.Param1, name, age, tags)
				return nil
			}))
			c.Assert(err, qt.IsNil, qt.Commentf("unable to create the reflection caller"))

			// Test the method works and actually gets called
			resp, err := method(context.Background(), &EncoreExampleWithPathParams{
				Payload: &ExampleParams{
					Param1: "this value",
					Param2: "ignored",
				},
				P0: "first param",
				P1: 1337,
				P2: []string{"a", "b", "c"},
			})
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.Equals, Void{}, qt.Commentf("api should return Void{}"))
			c.Assert(calledWith, qt.Equals, "this value - first param=1337 [a b c]", qt.Commentf("api should have been called with the correct value"))
		})

		c.Run("api that returns a response", func(c *qt.C) {
			c.Parallel()

			returnNil := false
			returnErr := false
			method, err := createReflectionCaller[*EncoreEmptyReq, *ExampleResponse](reflect.ValueOf(func(ctx context.Context) (*ExampleResponse, error) {
				if returnErr {
					return nil, errors.New("test error")
				}
				if returnNil {
					return nil, nil
				}
				return &ExampleResponse{Value: "hello"}, nil
			}))
			c.Assert(err, qt.IsNil, qt.Commentf("unable to create the reflection caller"))

			// Test the method works and actually gets called
			resp, err := method(context.Background(), nil)
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.DeepEquals, &ExampleResponse{Value: "hello"}, qt.Commentf("api should return the correct response"))

			// Test nil, nil is handled correctly
			returnNil = true
			resp, err = method(context.Background(), nil)
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.IsNil, qt.Commentf("api should return nil"))

			// Test error is returned correctly
			returnErr = true
			resp, err = method(context.Background(), nil)
			c.Assert(err, qt.ErrorMatches, "test error", qt.Commentf("api should return an error"))
			c.Assert(resp, qt.IsNil, qt.Commentf("api should return nil"))
		})

		c.Run("test receiver methods work too", func(c *qt.C) {
			c.Parallel()

			obj := &ExampleMockService{value: "hello", calledWith: "<not called>"}
			method, err := createReflectionCaller[*EncoreExampleParamsRequest, *ExampleResponse](reflect.ValueOf(obj.ExampleMethod))
			c.Assert(err, qt.IsNil, qt.Commentf("unable to create the reflection caller"))

			// Test the method works and actually gets called, and can reference the struct instance
			resp, err := method(context.Background(), &EncoreExampleParamsRequest{
				Payload: &ExampleParams{
					Param1: "this value",
					Param2: "ignored",
				},
			})
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.DeepEquals, &ExampleResponse{Value: "hello"}, qt.Commentf("api should return the correct response"))
			c.Assert(obj.calledWith, qt.Equals, "this value - ignored", qt.Commentf("api should have been called with the correct value"))

			// Test nil, nil is handled correctly
			resp, err = method(context.Background(), &EncoreExampleParamsRequest{Payload: &ExampleParams{
				Param1: "foobar",
				Param2: "nil",
			}})
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.IsNil, qt.Commentf("api should return nil"))
			c.Assert(obj.calledWith, qt.Equals, "foobar - nil", qt.Commentf("api should have been called with the correct value"))

			// Test error is returned correctly
			resp, err = method(context.Background(), &EncoreExampleParamsRequest{Payload: &ExampleParams{
				Param1: "error",
				Param2: "test error value",
			}})
			c.Assert(err, qt.ErrorMatches, "test error value", qt.Commentf("api should return an error"))
			c.Assert(resp, qt.IsNil, qt.Commentf("api should return nil"))
			c.Assert(obj.calledWith, qt.Equals, "error - test error value", qt.Commentf("api should have been called with the correct value"))

			// Test nil request is handled correctly
			resp, err = method(context.Background(), &EncoreExampleParamsRequest{Payload: nil})
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.DeepEquals, &ExampleResponse{Value: "you gave me nil!"}, qt.Commentf("api should return the correct response"))
			c.Assert(obj.calledWith, qt.Equals, "got nil", qt.Commentf("api should have been called with the correct value"))

			// And finally test a nil Encore internal param is handled correctly
			obj.calledWith = "<not called>"
			resp, err = method(context.Background(), nil)
			c.Assert(err, qt.IsNil, qt.Commentf("didn't expect an error form the api"))
			c.Assert(resp, qt.DeepEquals, &ExampleResponse{Value: "you gave me nil!"}, qt.Commentf("api should return the correct response"))
			c.Assert(obj.calledWith, qt.Equals, "got nil", qt.Commentf("api should have been called with the correct value"))
		})
	})
}

type ExampleMockService struct {
	value      string
	calledWith string
}

func (e *ExampleMockService) ExampleMethod(_ context.Context, p *ExampleParams) (*ExampleResponse, error) {
	if p == nil {
		e.calledWith = "got nil"
		return &ExampleResponse{Value: "you gave me nil!"}, nil
	}

	e.calledWith = fmt.Sprintf("%s - %s", p.Param1, p.Param2)

	if p.Param1 == "error" {
		return nil, errors.New(p.Param2)
	} else if p.Param2 == "nil" {
		return nil, nil
	}
	return &ExampleResponse{Value: e.value}, nil
}

type EncoreEmptyReq struct{}

type EncoreExampleParamsRequest struct {
	Payload *ExampleParams
}

// Note: this is how Encore generates the request struct for an API
//
// The parameter type is always called `Payload` and placed first
// then each path parameter is added in order after that
type EncoreExampleWithPathParams struct {
	Payload *ExampleParams
	P0      string
	P1      int
	P2      []string
}

type ExampleParams struct {
	Param1 string
	Param2 string
}

type ExampleResponse struct {
	Value string
}
