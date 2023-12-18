package util

import (
	"bytes"
	"regexp"

	json "github.com/json-iterator/go"
	"yunion.io/x/jsonutils"
)

type JsonSnakeCase struct {
	Value interface{}
}

func (c JsonSnakeCase) MarshalJSON() ([]byte, error) {
	var keyMatchRegex = regexp.MustCompile(`\"(\w+)\":`)
	var wordBarrierRegex = regexp.MustCompile(`(\w)([A-Z])`)
	marshalled, err := json.Marshal(c.Value)
	converted := keyMatchRegex.ReplaceAllFunc(
		marshalled,
		func(match []byte) []byte {
			return bytes.ToLower(wordBarrierRegex.ReplaceAll(
				match,
				[]byte(`${1}_${2}`),
			))
		},
	)
	return converted, err
}

func GetValueByKeys[T any](data jsonutils.JSONObject, keys ...string) T {
	var t T
	for _, key := range keys {
		value, err := data.Get(key)
		if err == nil && value != nil {
			return value.Interface().(T)
		}
	}
	return t
}
