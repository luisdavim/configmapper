package config

import (
	"fmt"
	"reflect"
	"strconv"
	"syscall"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StringToMetaV1DurationHookFunc returns a DecodeHookFunc that converts
// strings to metav1.Duration.
func StringToMetaV1DurationHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeFor[metav1.Duration]() {
			return data, nil
		}

		res, err := time.ParseDuration(data.(string))
		// Convert it by parsing
		return metav1.Duration{Duration: res}, err
	}
}

func StringToSyscallSignalHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeFor[syscall.Signal]() {
			return data, nil
		}

		s := data.(string)
		if s == "" {
			return syscall.SIGHUP, nil
		}

		res := unix.SignalNum(s)
		if res == 0 {
			if i, err := strconv.Atoi(s); err == nil {
				return syscall.Signal(i), nil
			}
			return res, fmt.Errorf("invalid signal name: %s", s)
		}

		return res, nil
	}
}

// DecodeHooks returns a DecoderConfigOption to override viper's default DecoderConfig.DecodeHook value
// to include the StringToMetaV1DurationHookFunc hook
func DecodeHooks() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		StringToMetaV1DurationHookFunc(),
		StringToSyscallSignalHookFunc(),
	))
}
