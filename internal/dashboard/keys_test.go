package dashboard

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestEachKeyIsBoundOnce(t *testing.T) {
	boundTo := map[string]string{}
	bindings := reflect.ValueOf(keys)
	for i := range bindings.NumField() {
		name, binding := bindings.Type().Field(i).Name, bindings.Field(i).Interface().(key.Binding)
		if len(binding.Keys()) == 0 {
			t.Errorf("%s has no keys", name)
		}
		for _, k := range binding.Keys() {
			if other, bound := boundTo[k]; bound {
				t.Errorf("%q is bound to both %s and %s", k, other, name)
			}
			boundTo[k] = name
		}
	}
}
