// Package validator menyediakan validasi struct yang konsisten dan berbahasa Indonesia.
package validator

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	playground "github.com/go-playground/validator/v10"
)

// Validator membungkus go-playground/validator untuk menghasilkan detail error API.
type Validator struct {
	validate *playground.Validate
}

// New membuat validator dengan perilaku required struct yang ketat.
func New() *Validator {
	validate := playground.New(playground.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return field.Name
		}
		return name
	})
	return &Validator{validate: validate}
}

// Validate memvalidasi struct dan mengembalikan pesan per field JSON.
func (validator *Validator) Validate(value any) map[string]string {
	err := validator.validate.Struct(value)
	if err == nil {
		return nil
	}
	validationErrors, ok := err.(playground.ValidationErrors)
	if !ok {
		return map[string]string{"request": "Data permintaan tidak dapat divalidasi."}
	}
	details := make(map[string]string, len(validationErrors))
	for _, fieldError := range validationErrors {
		field := fieldError.Field()
		if _, exists := details[field]; !exists {
			details[field] = indonesiaMessage(fieldError)
		}
	}
	return details
}

func indonesiaMessage(fieldError playground.FieldError) string {
	field := fieldError.Field()
	label := upperFirst(field)
	switch fieldError.Tag() {
	case "required":
		return label + " wajib diisi."
	case "email":
		return "Format " + field + " tidak valid."
	case "min":
		return fmt.Sprintf("%s minimal %s karakter.", label, fieldError.Param())
	case "max":
		return fmt.Sprintf("%s maksimal %s karakter.", label, fieldError.Param())
	case "oneof":
		return label + " memiliki nilai yang tidak didukung."
	default:
		return label + " tidak valid."
	}
}

func upperFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
