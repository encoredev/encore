package userconfig

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
)

func (c *Config) GetByKey(key string) (v Value, ok bool) {
	val := reflect.ValueOf(c).Elem()
	desc, ok := descs[key]
	if !ok {
		return Value{}, false
	}

	f := val.FieldByName(desc.FieldName)
	if !f.IsValid() {
		return Value{}, false
	}

	return Value{Val: f.Interface(), Type: desc.Type}, true
}

func (c *Config) Render() string {
	var buf strings.Builder
	for _, key := range configKeys {
		v, ok := c.GetByKey(key)
		if !ok {
			continue
		}
		buf.WriteString(fmt.Sprintf("%s: %s\n", key, v))
	}
	return buf.String()
}

var configKeys = (func() []string {
	keys := slices.Collect(maps.Keys(descs))
	sort.Strings(keys)
	return keys
})()

func GetType(key string) (Type, bool) {
	typ, ok := descs[key]
	return typ.Type, ok
}

func Keys() []string {
	return configKeys
}
