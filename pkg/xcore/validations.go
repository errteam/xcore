package xcore

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is the shared validator instance
var validate = validator.New()

// ValidationError represents a single field validation error
// This format is optimized for frontend consumption
type ValidationError struct {
	Field   string `json:"field"`           // camelCase field name
	Message string `json:"message"`         // User-friendly message
	Code    string `json:"code"`            // Machine-readable error code
	Param   string `json:"param,omitempty"` // Validation parameter (e.g., min length)
}

// ValidationErrors represents a collection of validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// FieldErrors is a map of field names to error messages
// This is the most frontend-friendly format for form validation
type FieldErrors map[string][]string

// ToFieldErrors converts ValidationErrors to FieldErrors map
// Example: {"email": ["Email is required"], "password": ["Too short"]}
func (ve *ValidationErrors) ToFieldErrors() FieldErrors {
	fieldErrors := make(FieldErrors)
	for _, err := range ve.Errors {
		fieldErrors[err.Field] = append(fieldErrors[err.Field], err.Message)
	}
	return fieldErrors
}

// ToFieldErrorMap converts ValidationErrors to a simple field->message map
// Useful when you only want the first error per field
func (ve *ValidationErrors) ToFieldErrorMap() map[string]string {
	fieldMap := make(map[string]string)
	for _, err := range ve.Errors {
		if _, exists := fieldMap[err.Field]; !exists {
			fieldMap[err.Field] = err.Message
		}
	}
	return fieldMap
}

// ParseValidationErrors parses Gin/validation errors into structured format
func ParseValidationErrors(err error) *ValidationErrors {
	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	if !asValidationError(err, &validationErrors) {
		// If it's not a validator error, return generic error
		return &ValidationErrors{
			Errors: []ValidationError{
				{
					Field:   "request",
					Message: err.Error(),
					Code:    "INVALID_FORMAT",
				},
			},
		}
	}

	errors := make([]ValidationError, 0, len(validationErrors))
	for _, ve := range validationErrors {
		errors = append(errors, ValidationError{
			Field:   formatField(ve.Field()),
			Message: getErrorMessage(ve),
			Code:    getErrorCode(ve),
			Param:   ve.Param(),
		})
	}

	return &ValidationErrors{Errors: errors}
}

// asValidationError is a helper to assert validator.ValidationErrors
func asValidationError(err error, target *validator.ValidationErrors) bool {
	errs, ok := err.(validator.ValidationErrors)
	if ok {
		*target = errs
		return true
	}
	return false
}

// formatField converts Go field names to camelCase for frontend
func formatField(field string) string {
	if len(field) == 0 {
		return field
	}
	return strings.ToLower(field[:1]) + field[1:]
}

// getErrorMessage returns a user-friendly error message based on the validation tag
func getErrorMessage(ve validator.FieldError) string {
	tag := ve.Tag()
	field := formatField(ve.Field())

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "required_if":
		return fmt.Sprintf("%s is required when %s is provided", field, ve.Param())
	case "required_unless":
		return fmt.Sprintf("%s is required unless %s is provided", field, ve.Param())
	case "required_with":
		return fmt.Sprintf("%s is required when %s is present", field, ve.Param())
	case "required_without":
		return fmt.Sprintf("%s is required when %s is not present", field, ve.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "http_url":
		return fmt.Sprintf("%s must be a valid HTTP URL", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, ve.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, ve.Param())
	case "min_len":
		return fmt.Sprintf("%s must be at least %s characters", field, ve.Param())
	case "max_len":
		return fmt.Sprintf("%s must be at most %s characters", field, ve.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, ve.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, ve.Param())
	case "numeric":
		return fmt.Sprintf("%s must be a number", field)
	case "number":
		return fmt.Sprintf("%s must be a number", field)
	case "alpha":
		return fmt.Sprintf("%s can only contain letters", field)
	case "alphanum":
		return fmt.Sprintf("%s can only contain letters and numbers", field)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, ve.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, ve.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, ve.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, ve.Param())
	case "eq":
		return fmt.Sprintf("%s must be equal to %s", field, ve.Param())
	case "ne":
		return fmt.Sprintf("%s must not be equal to %s", field, ve.Param())
	case "startswith":
		return fmt.Sprintf("%s must start with %s", field, ve.Param())
	case "endswith":
		return fmt.Sprintf("%s must end with %s", field, ve.Param())
	case "contains":
		return fmt.Sprintf("%s must contain %s", field, ve.Param())
	case "containsany":
		return fmt.Sprintf("%s must contain one of: %s", field, ve.Param())
	case "excludes":
		return fmt.Sprintf("%s must not contain %s", field, ve.Param())
	case "datetime":
		return fmt.Sprintf("%s must be a valid datetime", field)
	case "boolean":
		return fmt.Sprintf("%s must be a boolean", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "uuid_rfc4122":
		return fmt.Sprintf("%s must be a valid RFC4122 UUID", field)
	case "uuid3":
		return fmt.Sprintf("%s must be a valid UUID3", field)
	case "uuid4":
		return fmt.Sprintf("%s must be a valid UUID4", field)
	case "uuid5":
		return fmt.Sprintf("%s must be a valid UUID5", field)
	case "ip":
		return fmt.Sprintf("%s must be a valid IP address", field)
	case "ipv4":
		return fmt.Sprintf("%s must be a valid IPv4 address", field)
	case "ipv6":
		return fmt.Sprintf("%s must be a valid IPv6 address", field)
	case "cidr":
		return fmt.Sprintf("%s must be a valid CIDR", field)
	case "hex":
		return fmt.Sprintf("%s must be a valid hexadecimal string", field)
	case "base64":
		return fmt.Sprintf("%s must be a valid base64 encoded string", field)
	case "base64url":
		return fmt.Sprintf("%s must be a valid base64url encoded string", field)
	case "hostname":
		return fmt.Sprintf("%s must be a valid hostname", field)
	case "fqdn":
		return fmt.Sprintf("%s must be a fully qualified domain name", field)
	case "credit_card":
		return fmt.Sprintf("%s must be a valid credit card number", field)
	case "isbn":
		return fmt.Sprintf("%s must be a valid ISBN", field)
	case "isbn10":
		return fmt.Sprintf("%s must be a valid ISBN10", field)
	case "isbn13":
		return fmt.Sprintf("%s must be a valid ISBN13", field)
	case "postcode_iso3166_alpha2":
		return fmt.Sprintf("%s must be a valid postal code", field)
	case "postcode_iso3166_alpha2_field":
		return fmt.Sprintf("%s must be a valid postal code for the country", field)
	case "latitude":
		return fmt.Sprintf("%s must be a valid latitude", field)
	case "longitude":
		return fmt.Sprintf("%s must be a valid longitude", field)
	case "file":
		return fmt.Sprintf("%s must be a valid file", field)
	case "image":
		return fmt.Sprintf("%s must be a valid image file", field)
	case "dir":
		return fmt.Sprintf("%s must be a valid directory", field)
	case "json":
		return fmt.Sprintf("%s must be a valid JSON string", field)
	case "jwt":
		return fmt.Sprintf("%s must be a valid JWT token", field)
	case "country_code":
		return fmt.Sprintf("%s must be a valid country code", field)
	case "currency":
		return fmt.Sprintf("%s must be a valid currency code", field)
	case "timezone":
		return fmt.Sprintf("%s must be a valid timezone", field)
	case "language":
		return fmt.Sprintf("%s must be a valid language code", field)
	case "locale":
		return fmt.Sprintf("%s must be a valid locale", field)
	case "bic":
		return fmt.Sprintf("%s must be a valid BIC/SWIFT code", field)
	default:
		return fmt.Sprintf("%s validation failed", field)
	}
}

// getErrorCode returns an error code based on the validation tag
func getErrorCode(ve validator.FieldError) string {
	tag := ve.Tag()

	switch tag {
	case "required":
		return "REQUIRED"
	case "required_if", "required_unless", "required_with", "required_without":
		return "REQUIRED_CONDITIONAL"
	case "email":
		return "INVALID_EMAIL"
	case "url", "http_url":
		return "INVALID_URL"
	case "min", "min_len":
		return "TOO_SHORT"
	case "max", "max_len":
		return "TOO_LONG"
	case "len":
		return "INVALID_LENGTH"
	case "oneof":
		return "INVALID_VALUE"
	case "numeric", "number":
		return "NOT_NUMERIC"
	case "alpha":
		return "INVALID_CHARACTERS"
	case "alphanum":
		return "INVALID_CHARACTERS"
	case "gt", "gte":
		return "VALUE_TOO_LOW"
	case "lt", "lte":
		return "VALUE_TOO_HIGH"
	case "eq":
		return "VALUE_MISMATCH"
	case "ne":
		return "VALUE_MATCH"
	case "startswith":
		return "INVALID_PREFIX"
	case "endswith":
		return "INVALID_SUFFIX"
	case "contains", "containsany":
		return "MISSING_SUBSTRING"
	case "excludes":
		return "FORBIDDEN_SUBSTRING"
	case "datetime":
		return "INVALID_DATETIME"
	case "boolean":
		return "NOT_BOOLEAN"
	case "uuid", "uuid_rfc4122", "uuid3", "uuid4", "uuid5":
		return "INVALID_UUID"
	case "ip", "ipv4", "ipv6":
		return "INVALID_IP"
	case "cidr":
		return "INVALID_CIDR"
	case "hex":
		return "INVALID_HEX"
	case "base64", "base64url":
		return "INVALID_BASE64"
	case "hostname", "fqdn":
		return "INVALID_HOSTNAME"
	case "credit_card":
		return "INVALID_CREDIT_CARD"
	case "isbn", "isbn10", "isbn13":
		return "INVALID_ISBN"
	case "postcode_iso3166_alpha2", "postcode_iso3166_alpha2_field":
		return "INVALID_POSTAL_CODE"
	case "latitude":
		return "INVALID_LATITUDE"
	case "longitude":
		return "INVALID_LONGITUDE"
	case "file":
		return "INVALID_FILE"
	case "image":
		return "INVALID_IMAGE"
	case "dir":
		return "INVALID_DIRECTORY"
	case "json":
		return "INVALID_JSON"
	case "jwt":
		return "INVALID_JWT"
	case "country_code":
		return "INVALID_COUNTRY_CODE"
	case "currency":
		return "INVALID_CURRENCY"
	case "timezone":
		return "INVALID_TIMEZONE"
	case "language":
		return "INVALID_LANGUAGE"
	case "locale":
		return "INVALID_LOCALE"
	case "bic":
		return "INVALID_BIC"
	default:
		return "VALIDATION_FAILED"
	}
}

// GetFieldErrors is a convenience function to get FieldErrors directly
func GetFieldErrors(err error) FieldErrors {
	ve := ParseValidationErrors(err)
	if ve == nil {
		return nil
	}
	return ve.ToFieldErrors()
}

// GetFirstError gets the first error message for a specific field
func (ve *ValidationErrors) GetFirstError(field string) string {
	for _, err := range ve.Errors {
		if err.Field == field {
			return err.Message
		}
	}
	return ""
}

// HasField checks if there are errors for a specific field
func (ve *ValidationErrors) HasField(field string) bool {
	for _, err := range ve.Errors {
		if err.Field == field {
			return true
		}
	}
	return false
}

// IsEmpty checks if there are no validation errors
func (ve *ValidationErrors) IsEmpty() bool {
	return ve == nil || len(ve.Errors) == 0
}

// Count returns the number of validation errors
func (ve *ValidationErrors) Count() int {
	if ve == nil {
		return 0
	}
	return len(ve.Errors)
}
