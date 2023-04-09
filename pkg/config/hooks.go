package config

import (
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StringToMetaV1DurationHookFunc returns a DecodeHookFunc that converts
// strings to metav1.Duration.
func StringToMetaV1DurationHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(metav1.Duration{}) {
			return data, nil
		}

		res, err := time.ParseDuration(data.(string))
		// Convert it by parsing
		return metav1.Duration{Duration: res}, err
	}
}

// DecodeHooks returns a DecoderConfigOption to override viper's default DecoderConfig.DecodeHook value
// to include the StringToMetaV1DurationHookFunc hook
func DecodeHooks() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		StringToMetaV1DurationHookFunc(),
	))
}
