package map2struct

import (
	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
	"gopkg.in/guregu/null.v4"
	"reflect"
	"time"
)

var timeLocation *time.Location

func init() {
	timeLocation, _ = time.LoadLocation("Asia/Shanghai")
}

func ToTimeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(time.Time{}) {
			return data, nil
		}

		switch f.Kind() {
		case reflect.String:
			return cast.ToTimeInDefaultLocationE(data, timeLocation)
		case reflect.Float64:
			return time.Unix(0, int64(data.(float64))*int64(time.Millisecond)), nil
		case reflect.Int64:
			return time.Unix(0, data.(int64)*int64(time.Millisecond)), nil
		default:
			return data, nil
		}
		// Convert it by parsing
	}
}

func ToDecimalHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {

		if t == reflect.TypeOf(decimal.Decimal{}) {
			if f.Kind() == reflect.Float64 {
				return decimal.NewFromFloat(data.(float64)), nil
			}

			if input := data.(string); input != "" {
				return decimal.NewFromString(data.(string))
			}
			return decimal.Decimal{}, nil
		}

		return data, nil
	}
}

func ToNullableStringHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t == reflect.TypeOf(null.String{}) {
			return null.StringFrom(data.(string)), nil
		}

		return data, nil
	}
}
