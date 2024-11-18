package infra

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/modern-go/reflect2"
)

type JSONPath string

func (j JSONPath) String() string {
	return string(j)
}

func (j JSONPath) Append(path JSONPath) JSONPath {
	return JSONPath(fmt.Sprintf("%s.%s", j, path))
}

func Ancestor[T any](a *validator) T {
	for a != nil {
		if v, ok := a.instance.(T); ok {
			return v
		}
		a = a.parent
	}
	var t T
	return t
}

type Predicate func() error

func Err(err string) Predicate {
	return func() error {
		return errors.New(err)
	}
}

func AnyNonZero[T comparable](v ...T) Predicate {
	return func() error {
		var zero T
		for _, val := range v {
			if zero != val {
				return nil
			}
		}
		return errors.New("Must not be empty")
	}
}

func OneOf[T comparable](v T, vals ...T) Predicate {
	return func() error {
		if !slices.Contains(vals, v) {
			return errors.New("Must be one of: " + fmt.Sprint(vals))
		}
		return nil
	}
}

func NotZero[T comparable](v T) Predicate {
	return func() error {
		var zero T
		if zero == v {
			return errors.New("Must not be empty")
		}
		return nil
	}
}

func NilOr[T any](v *T, predFns ...func(T) Predicate) Predicate {
	return func() error {
		if v == nil {
			return nil
		}
		for _, predFn := range predFns {
			if err := predFn(*v)(); err != nil {
				return err
			}
		}
		return nil
	}
}

func Between[T cmp.Ordered](from, to T) func(T) Predicate {
	return func(t T) Predicate {
		return func() error {
			if t < from || t > to {
				return fmt.Errorf("Must be between %v and %v (inclusive)", from, to)
			}
			return nil
		}
	}
}

func GreaterOrEqual[T cmp.Ordered](than T) func(T) Predicate {
	return func(t T) Predicate {
		return func() error {
			if t < than {
				return fmt.Errorf("Must be greater than or equal to %v", than)
			}
			return nil
		}
	}
}

type EnvDesc struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

func Validate(v Validatable) (map[JSONPath]EnvDesc, map[JSONPath]error) {
	root := &validator{instance: v, envDescs: make(map[string]EnvDesc), errors: make(map[string]error)}
	v.Validate(root)
	return root.Collect()
}

func ValidateChildList[T Validatable](v *validator, name string, items []T) {
	for i, item := range items {
		v.ValidateChild(fmt.Sprintf("%s[%d]", name, i), item)
	}
}

func ValidateChildMap[T Validatable](v *validator, name string, items map[string]T) {
	for k, item := range items {
		v.ValidateChild(fmt.Sprintf("%s.%s", name, k), item)
	}
}

type validator struct {
	parent   *validator
	name     string
	instance Validatable
	envDescs map[string]EnvDesc
	errors   map[string]error
	children []*validator
}

func (val *validator) Collect() (map[JSONPath]EnvDesc, map[JSONPath]error) {
	envDescs := map[JSONPath]EnvDesc{}
	errors := map[JSONPath]error{}
	for k, v := range val.envDescs {
		envDescs[JSONPath(val.name).Append(JSONPath(k))] = v
	}
	for k, v := range val.errors {
		errors[JSONPath(val.name).Append(JSONPath(k))] = v
	}
	for _, child := range val.children {
		childEnvDescs, childErrors := child.Collect()
		for k, v := range childEnvDescs {
			envDescs[JSONPath(val.name).Append(k)] = v
		}
		for k, v := range childErrors {
			errors[JSONPath(val.name).Append(k)] = v
		}
	}
	return envDescs, errors
}

func (v *validator) ValidatePtrEnvRef(name string, ref *EnvString, envDesc string, predFn func(string) Predicate) {
	if ref == nil {
		return
	}
	v.ValidateEnvString(name, *ref, envDesc, predFn)
}

func (v *validator) ValidateEnvRef(name string, ref EnvRef, envDesc string) {
	v.envDescs[name] = ref.Describe(envDesc)
}

func (v *validator) ValidateEnvString(name string, envStr EnvString, envDesc string, predFn func(string) Predicate) {
	if envStr.IsEnvRef() {
		v.ValidateEnvRef(name, *envStr.Env, envDesc)
	} else if predFn != nil {
		v.ValidateField(name, predFn(envStr.Str))
	}
}

func (v *validator) ValidateField(name string, pred Predicate) {
	err := pred()
	if err == nil {
		return
	}
	v.errors[name] = err
}

func (v *validator) ValidateChild(name string, instance Validatable) {
	if reflect2.IsNil(instance) {
		return
	}
	child := &validator{name: name, instance: instance, parent: v, envDescs: make(map[string]EnvDesc), errors: make(map[string]error)}
	instance.Validate(child)
	v.children = append(v.children, child)
}

type Validatable interface {
	Validate(validator *validator)
}
