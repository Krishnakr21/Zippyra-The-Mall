package validator

import (
	"strconv"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Validate checks a struct for validation tags.
func Validate(s interface{}) error {
	return validate.Struct(s)
}

// ValidateEAN13 checks if a string is a valid EAN-13 barcode.
func ValidateEAN13(ean string) bool {
	if len(ean) != 13 {
		return false
	}
	
	for _, r := range ean {
		if r < '0' || r > '9' {
			return false
		}
	}

	sum := 0
	for i := 0; i < 12; i++ {
		digit, _ := strconv.Atoi(string(ean[i]))
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}

	checkDigit := (10 - (sum % 10)) % 10
	actualCheckDigit, _ := strconv.Atoi(string(ean[12]))

	return checkDigit == actualCheckDigit
}
