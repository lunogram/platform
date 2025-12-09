package http

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nyaruka/phonenumbers"
)

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*$")

func init() {
	openapi3.DefineStringFormatValidator("email", openapi3.NewCallbackValidator(func(value string) error {
		if !emailRegex.MatchString(value) {
			return fmt.Errorf("invalid email format")
		}
		return nil
	}))

	openapi3.DefineStringFormatValidator("phone", openapi3.NewCallbackValidator(func(value string) error {
		num, err := phonenumbers.Parse(value, "")
		if err != nil {
			return err
		}

		if !phonenumbers.IsValidNumber(num) {
			return errors.New("phone number must be in E.164 format (e.g., +14155552671)")
		}

		if phonenumbers.Format(num, phonenumbers.E164) != value {
			return errors.New("phone number must be in E.164 format (e.g., +14155552671)")
		}

		return nil
	}))
}
