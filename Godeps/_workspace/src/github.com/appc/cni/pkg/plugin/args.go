package plugin

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
)

func LoadArgs(args string, container interface{}) error {
	if args == "" {
		return nil
	}

	containerValue := reflect.ValueOf(container)

	pairs := strings.Split(args, ";")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			return fmt.Errorf("ARGS: invalid pair %q", pair)
		}
		keyString := kv[0]
		valueString := kv[1]
		keyField := containerValue.Elem().FieldByName(keyString)
		if !keyField.IsValid() {
			return fmt.Errorf("ARGS: invalid key %q", keyString)
		}
		u := keyField.Addr().Interface().(encoding.TextUnmarshaler)
		err := u.UnmarshalText([]byte(valueString))
		if err != nil {
			return fmt.Errorf("ARGS: error parsing value of pair %q: %v)", pair, err)
		}
	}
	return nil
}
