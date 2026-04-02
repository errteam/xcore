package xcore

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.SetTagName("validate")
}

type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Tag     string      `json:"tag,omitempty"`
	Value   interface{} `json:"value,omitempty"`
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	var errs []string
	for _, e := range v {
		errs = append(errs, e.Field+": "+e.Message)
	}
	return strings.Join(errs, ", ")
}

func (v ValidationErrors) ToResponseErrors() []ResponseError {
	var respErrors []ResponseError
	for _, e := range v {
		respErrors = append(respErrors, ResponseError{
			Field:   e.Field,
			Message: e.Message,
		})
	}
	return respErrors
}

func GetValidationErrors(err error) ValidationErrors {
	var validationErrs ValidationErrors

	if valErrs, ok := err.(validator.ValidationErrors); ok {
		for _, err := range valErrs {
			field := strings.Split(err.Field(), ".")[0]
			field = toSnakeCase(field)

			validationErrs = append(validationErrs, ValidationError{
				Field:   field,
				Message: err.Tag(),
				Tag:     err.Tag(),
				Value:   err.Value(),
			})
		}
	}

	if len(validationErrs) == 0 {
		if errs, ok := err.(ValidationErrors); ok {
			validationErrs = errs
		}
	}

	return validationErrs
}

func toSnakeCase(str string) string {
	var result strings.Builder
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

type Validator struct {
	validate *validator.Validate
}

func NewValidator() *Validator {
	return &Validator{
		validate: validator.New(),
	}
}

func (v *Validator) ValidateStruct(s interface{}) error {
	if err := v.validate.Struct(s); err != nil {
		return err
	}
	return nil
}

func (v *Validator) ValidateVar(field interface{}, tag string) error {
	return v.validate.Var(field, tag)
}

func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}

func Validate(s interface{}) error {
	if s == nil {
		return nil
	}
	return validate.Struct(s)
}

func ValidateVar(field interface{}, tag string) error {
	return validate.Var(field, tag)
}

type Binding interface {
	Name() string
	Bind(*http.Request, interface{}) error
}

type JSONBinding struct{}

func (j *JSONBinding) Name() string {
	return "json"
}

func (j *JSONBinding) Bind(req *http.Request, v interface{}) error {
	if req.Body == nil {
		return nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return &ValidationErrors{{Field: "body", Message: "Failed to read request body"}}
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return &ValidationErrors{{Field: "body", Message: "Invalid JSON format"}}
	}

	return nil
}

type QueryBinding struct{}

func (q *QueryBinding) Name() string {
	return "query"
}

func (q *QueryBinding) Bind(req *http.Request, v interface{}) error {
	values := req.URL.Query()

	data := make(map[string]interface{})
	for k, v := range values {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	jsonData, _ := json.Marshal(data)
	return json.Unmarshal(jsonData, v)
}

type FormBinding struct{}

func (f *FormBinding) Name() string {
	return "form"
}

func (f *FormBinding) Bind(req *http.Request, v interface{}) error {
	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		return f.bindMultipart(req, v)
	}

	return f.bindURLEncoded(req, v)
}

func (f *FormBinding) bindMultipart(req *http.Request, v interface{}) error {
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		return &ValidationErrors{{Field: "form", Message: "Failed to parse multipart form"}}
	}

	data := make(map[string]interface{})

	for k, v := range req.Form {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	jsonData, _ := json.Marshal(data)
	return json.Unmarshal(jsonData, v)
}

func (f *FormBinding) bindURLEncoded(req *http.Request, v interface{}) error {
	if err := req.ParseForm(); err != nil {
		return &ValidationErrors{{Field: "form", Message: "Failed to parse form"}}
	}

	data := make(map[string]interface{})

	for k, v := range req.Form {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	jsonData, _ := json.Marshal(data)
	return json.Unmarshal(jsonData, v)
}

var (
	JSON   Binding = &JSONBinding{}
	Query  Binding = &QueryBinding{}
	Form   Binding = &FormBinding{}
	Header Binding = &HeaderBinding{}
)

type HeaderBinding struct{}

func (h *HeaderBinding) Name() string {
	return "header"
}

func (h *HeaderBinding) Bind(req *http.Request, v interface{}) error {
	values := make(map[string]string)

	for k, v := range req.Header {
		if len(v) > 0 {
			values[strings.ToLower(k)] = v[0]
		}
	}

	jsonData, _ := json.Marshal(values)
	return json.Unmarshal(jsonData, v)
}

type CustomValidator func(interface{}) error

func RegisterCustomValidator(tag string, fn CustomValidator) error {
	return validate.RegisterValidation(tag, func(fl validator.FieldLevel) bool {
		return fn(fl.Field().Interface()) == nil
	})
}

func ValidateContext(ctx context.Context, s interface{}) error {
	return validate.StructCtx(ctx, s)
}

type ValidatorMiddleware struct {
	logger *Logger
}

func NewValidatorMiddleware(logger *Logger) *ValidatorMiddleware {
	return &ValidatorMiddleware{logger: logger}
}
