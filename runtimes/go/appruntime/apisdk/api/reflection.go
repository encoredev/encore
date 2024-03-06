package api

import (
	"context"
	"reflect"
	"strings"

	"encore.dev/beta/errs"
)

var (
	voidType    = reflect.TypeOf((*Void)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

type reflectedAPIMethod[Req any, Resp any] func(ctx context.Context, req Req) (Resp, error)

// createReflectionCaller takes a reflected function which should match the signature of an API method which is
// described by the Req and Resp types, and returns a function which can be used to call that function using
// the provided request and response types.
//
// It will return an error if the function does not match the expected signature.
func createReflectionCaller[Req any, Resp any](function reflect.Value) (reflectedAPIMethod[Req, Resp], error) {
	// Sanity check that the function is a function
	if function.Kind() != reflect.Func {
		return nil, errs.B().Code(errs.Internal).Msgf("expected a function, got %s", function.Kind()).Err()
	}
	typ := function.Type()

	// Get the request type
	requestType := reflect.TypeOf((*Req)(nil)).Elem()
	if requestType.Kind() != reflect.Pointer {
		return nil, errs.B().Code(errs.Internal).Msgf("expected request type to be a pointer, got %s", requestType.Kind()).Err()
	}
	requestType = requestType.Elem()
	if requestType.Kind() != reflect.Struct {
		return nil, errs.B().Code(errs.Internal).Msgf("expected request type to be a struct, got %s", requestType.Kind()).Err()
	}

	expectedNumInParams := 1 + requestType.NumField()

	// Build up the list of parameters we expect in the order they should be passed to the function
	expectedParamTypes := make([]reflect.Type, 1, expectedNumInParams)
	paramFieldIndexes := make([]int, 1, expectedNumInParams)

	// All API's must always have a context as the first parameter
	expectedParamTypes[0] = contextType
	paramFieldIndexes[0] = -1 // -1 means it's not a field on the request struct

	// Now dynamically add the rest of the parameters both the payload and any path parameters
	var payloadType reflect.Type
	var payloadFieldIndex int
	for i := 0; i < requestType.NumField(); i++ {
		if requestType.Field(i).Name == "Payload" {
			payloadType = requestType.Field(i).Type
			payloadFieldIndex = i
		} else {
			expectedParamTypes = append(expectedParamTypes, requestType.Field(i).Type)
			paramFieldIndexes = append(paramFieldIndexes, i)
		}
	}
	if payloadType != nil {
		expectedParamTypes = append(expectedParamTypes, payloadType)
		paramFieldIndexes = append(paramFieldIndexes, payloadFieldIndex)
	}

	// Check the number of parameters is correct, if not return an error
	numInParams := typ.NumIn()
	if numInParams != expectedNumInParams {
		expectedParams := make([]string, expectedNumInParams)
		expectedParams[0] = "context.Context"
		for i := 0; i < requestType.NumField(); i++ {
			expectedParams[i+1] = requestType.Field(i).Type.String()
		}

		return nil, errs.B().Code(errs.Internal).Msgf("expected %d parameters (%s), got %d parameters", expectedNumInParams, strings.Join(expectedParams, ", "), numInParams).Err()
	}

	// Check all the parameters are of the correct type, if not return an error
	for i, expected := range expectedParamTypes {
		actual := typ.In(i)
		if actual != expected {
			return nil, errs.B().Code(errs.Internal).Msgf("expected parameter %d to be %s, got %s", i+1, expected, actual).Err()
		}
	}

	// If Resp is of type Void, then we expect 1 return value, otherwise 2
	responseType := reflect.TypeOf((*Resp)(nil)).Elem()
	numReturnValues := typ.NumOut()
	isVoidResponse := responseType == voidType
	var errResponseIdx int

	if isVoidResponse {
		errResponseIdx = 0

		if numReturnValues != 1 {
			return nil, errs.B().Code(errs.Internal).Msgf("expected one return value (error), got %d return values", numReturnValues).Err()
		}

		if typ.Out(0) != errorType {
			return nil, errs.B().Code(errs.Internal).Msgf("expected the return value to be an error, got %s", typ.Out(0)).Err()
		}
	} else {
		errResponseIdx = 1

		if numReturnValues != 2 {
			return nil, errs.B().Code(errs.Internal).Msgf("expected two return values (%s, error), got %d return values", responseType, numReturnValues).Err()
		}

		if typ.Out(0) != responseType {
			return nil, errs.B().Code(errs.Internal).Msgf("expected first return value to be %s, got %s", responseType, typ.Out(0)).Err()
		}

		if typ.Out(1) != errorType {
			return nil, errs.B().Code(errs.Internal).Msgf("expected second return value to be an error, got %s", typ.Out(1)).Err()
		}
	}

	return func(ctx context.Context, req Req) (respObj Resp, respErr error) {
		inParams := make([]reflect.Value, 1, expectedNumInParams)
		inParams[0] = reflect.ValueOf(ctx)

		// If we have parameters on the request, then we need to pass them to the function
		reqValue := reflect.ValueOf(req)
		if !reqValue.IsNil() {
			reqValue = reqValue.Elem() // deference the pointer that Encore always sets here

			for i := 1; i < expectedNumInParams; i++ {
				inParams = append(inParams, reqValue.Field(paramFieldIndexes[i]))
			}
		} else {
			// If we don't have an `EncoreInteral_FoobarRequest` object, then we need to pass in the zero value for each
			// parameter - this shouldn't happen as all Encore generated API calls will always pass in a request object
			for i := 1; i < expectedNumInParams; i++ {
				inParams = append(inParams, reflect.Zero(expectedParamTypes[i]))
			}
		}

		outParams := function.Call(inParams)

		// These two casts are safe because we've already checked the types above
		respParam := outParams[0]
		if !isVoidResponse && respParam.IsValid() && !respParam.IsZero() {
			respObj = respParam.Interface().(Resp)
		}

		outErr := outParams[errResponseIdx]
		if outErr.IsValid() && !outErr.IsZero() {
			respErr = outErr.Interface().(error)
		}
		return
	}, nil
}
