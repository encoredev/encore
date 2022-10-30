package et

import (
	"reflect"

	"encore.dev/config"
)

// SetCfg changes the value of cfg to newValue within the current test and any subtests.
// Other tests running will not be affected.
//
// It does not support setting slices and panics if given a config value that is a slice.
func SetCfg[T any](cfg config.Value[T], newValue T) {
	rt := reflect.TypeOf(newValue)
	switch rt.Kind() {
	case reflect.Slice:
		panic("et.SetCfg does not support setting slices")
	case reflect.Array:
		panic("et.SetCfg does not support setting arrays")
	}

	config.SetValueForTest[T](cfg, newValue)
}
