package utils

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"
)

// MarshalFields creates a map[string]string of fields in `msg` tagged with `tag`. The name of the tag
// will be used as map key, and values are converted to strings using fmt.Sprintf. Pointers will be dereferenced
// and ignored if nil. Only basic types (bool, numeric, string) and pointers to those types are supported fields.
// Fields without a tag will not be marshalled. `msg` must be a struct or pointer to a struct
func MarshalFields[T any](msg T, tag string) (map[string]string, error) {
	// Create a map to return
	rtn := map[string]string{}
	msgVal := reflect.ValueOf(msg)
	// Dereference the input msg
	for msgVal.Kind() == reflect.Ptr {
		msgVal = msgVal.Elem()
	}
	// Only support structs, or pointers to structs
	if msgVal.Kind() != reflect.Struct {
		return nil, errors.New("pubsub messages must be structs or a pointer to struct")
	}

	// Iterate through the message fields and look for `tag` tags, marshal if found
	for i := 0; i < msgVal.NumField(); i++ {
		msgField := msgVal.Field(i)
		if name, ok := msgVal.Type().Field(i).Tag.Lookup(tag); ok {
			isNil := false
			// We need to dereference pointers to get the value
			for msgField.Kind() == reflect.Pointer {
				// If it's a nil pointer we want to skip the field
				if msgField.IsNil() {
					isNil = true
					break
				}
				msgField = msgField.Elem()
			}
			if isNil {
				continue
			}
			// if the dereferenced type is not a basic type, return an error
			if msgField.Kind() >= reflect.Array && msgField.Kind() != reflect.String {
				return nil, errors.New(fmt.Sprintf("unsupported kind: %s", msgField.Kind()))
			}
			// serialize the value using string formatting
			rtn[name] = fmt.Sprintf("%v", msgField.Interface())
		}
	}
	return rtn, nil
}

// UnmarshalFields copies values from the attrs map to val the struct. The attrs key to copy the value from is
// retrieved from the name of the field tag with key `tag`. The string value is parsed to the target type before
// being assigned. Invalid values will return an error. Only basic types (bool, numeric, string) and pointers
// to those types are supported fields. Fields without a tag will not be touched. `val` must be a pointer to a struct,
// otherwise we cannot populate the fields
func UnmarshalFields[T any](attrs map[string]string, val T, tag string) error {
	v := reflect.ValueOf(val)
	// target type must be a pointer for us to set fields
	if v.Kind() != reflect.Pointer {
		return errors.New("target must be a pointer to a struct")
	}
	// Dereference the pointer
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	// We only support structs for now
	if v.Kind() != reflect.Struct {
		return errors.New(fmt.Sprintf("unsupported kind: %s", v.Kind()))
	}

	// Loop through the fields of the struct and look for `tag`s
	for i := 0; i < v.NumField(); i++ {
		if name, ok := v.Type().Field(i).Tag.Lookup(tag); ok {
			// If the attributes contain an element with the tag name, unmarshal it into target
			if attrVal, ok := attrs[name]; ok {
				dataField := v.Field(i)
				dataType := dataField.Type()
				ptrDepth := 0
				// Dereference the type (we can't use the value here because it might be nil)
				// Keep track of the pointer depth which is used later to create a pointer to
				// the value
				for ; dataType.Kind() == reflect.Ptr; ptrDepth++ {
					dataType = dataType.Elem()
				}
				// Parse and set the value. If the target type is a pointer, we need to
				// create a pointer. We use the type specific setter (e.g. SetInt) to handle
				// int bitsize conversions, and the generic setter (Set) to set pointers
				switch dataType.Kind() {
				case reflect.String:
					setValue(attrVal, ptrDepth, dataField, reflect.Value.SetString)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					val, err := strconv.ParseInt(attrVal, 10, 64)
					if err != nil {
						return err
					}
					setValue(val, ptrDepth, dataField, reflect.Value.SetInt)
				case reflect.Float32, reflect.Float64:
					val, err := strconv.ParseFloat(attrVal, 64)
					if err != nil {
						return err
					}
					setValue(val, ptrDepth, dataField, reflect.Value.SetFloat)
				case reflect.Bool:
					val, err := strconv.ParseBool(attrVal)
					if err != nil {
						return err
					}
					setValue(val, ptrDepth, dataField, reflect.Value.SetBool)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					val, err := strconv.ParseUint(attrVal, 10, 64)
					if err != nil {
						return err
					}
					setValue(val, ptrDepth, dataField, reflect.Value.SetUint)
				case reflect.Complex64, reflect.Complex128:
					val, err := strconv.ParseComplex(attrVal, 128)
					if err != nil {
						return err
					}
					setValue(val, ptrDepth, dataField, reflect.Value.SetComplex)
				default:
					return errors.New(fmt.Sprintf("unsupported kind: %s", dataField.Kind()))
				}
			}
		}
	}
	return nil
}

// setValue assigns a value or a (nested) pointer to a value to f
func setValue[T any](val T, ptrDepth int, f reflect.Value, valSetter func(reflect.Value, T)) {
	// if the type is not a pointer, we can set the value directly on the field
	if ptrDepth == 0 {
		valSetter(f, val)
	} else {
		// otherwise we need to create a stack of `ptrDepth` pointers and assign val
		// to a new value instance
		root := f.Type()
		// Find the dereferenced type
		for root.Kind() == reflect.Pointer {
			root = root.Elem()
		}
		// Create a new value of the dereferenced type (and dereference it)
		rval := reflect.New(root).Elem()
		// Set the content of the new Value
		valSetter(rval, val)
		// Wrap it in pointers and set the target field to the wrapped vaue
		f.Set(pointify(rval, ptrDepth))
	}
}

// pointify wraps a value in `ptrDepth` pointers
func pointify(rval reflect.Value, ptrDepth int) reflect.Value {
	for i := 0; i < ptrDepth; i++ {
		v := reflect.New(rval.Type())
		v.Elem().Set(rval)
		rval = v
	}
	return rval
}

// GetDelay returns whether a message should be retried and if so the backoff duration based on the
// configuration in the RetryPolicy
func GetDelay(maxRetries int, minDelay, maxDelay time.Duration, attempt uint16) (retry bool, delay time.Duration) {
	if maxRetries != -1 &&
		int(attempt) >= maxRetries {
		return false, 0
	}
	if maxDelay < minDelay {
		return true, maxDelay
	}
	delay = time.Duration(math.Max(float64(1*time.Second), float64(minDelay))) // delay at least 1 second

	for i := uint16(0); i < attempt; i++ {
		delay *= 2
		if delay > maxDelay {
			return true, maxDelay
		}
	}
	return true, delay
}
