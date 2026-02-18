package xcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Common content types
const (
	ContentTypeJSON              = "application/json"
	ContentTypeXML               = "application/xml"
	ContentTypeForm              = "application/x-www-form-urlencoded"
	ContentTypeMultipartForm     = "multipart/form-data"
	ContentTypeText              = "text/plain"
	ContentTypeHTML              = "text/html"
	ContentTypeOctetStream       = "application/octet-stream"
	ContentTypeJavaScript        = "application/javascript"
	ContentTypeCSS               = "text/css"
	ContentTypePNG               = "image/png"
	ContentTypeJPEG              = "image/jpeg"
	ContentTypeGIF               = "image/gif"
	ContentTypeSVG               = "image/svg+xml"
	ContentTypePDF               = "application/pdf"
	ContentTypeZIP               = "application/zip"
	ContentTypeGZIP              = "application/gzip"
	ContentTypeBinaryOctetStream = "binary/octet-stream"
)

// Common header names
const (
	HeaderContentType     = "Content-Type"
	HeaderAccept          = "Accept"
	HeaderAuthorization   = "Authorization"
	HeaderUserAgent       = "User-Agent"
	HeaderXRequestID      = "X-Request-ID"
	HeaderXForwardedFor   = "X-Forwarded-For"
	HeaderXRealIP         = "X-Real-IP"
	HeaderContentLength   = "Content-Length"
	HeaderCacheControl    = "Cache-Control"
	HeaderETag            = "ETag"
	HeaderLastModified    = "Last-Modified"
	HeaderIfNoneMatch     = "If-None-Match"
	HeaderIfModifiedSince = "If-Modified-Since"
)

// Common file extensions
const (
	ExtJSON = ".json"
	ExtXML  = ".xml"
	ExtCSV  = ".csv"
	ExtTXT  = ".txt"
	ExtPDF  = ".pdf"
	ExtZIP  = ".zip"
	ExtPNG  = ".png"
	ExtJPEG = ".jpg"
	ExtGIF  = ".gif"
	ExtSVG  = ".svg"
)

type RequestBuilder struct {
	request *http.Request
}

func NewRequestBuilder(r *http.Request) *RequestBuilder {
	return &RequestBuilder{request: r}
}

// ParseJSON parses and validates JSON request body
func (rb *RequestBuilder) ParseJSON(v interface{}) error {
	if err := json.NewDecoder(rb.request.Body).Decode(v); err != nil {
		return NewAppErrorWithErr(ErrInvalidFormat, "Invalid JSON format", err)
	}
	if err := validate.Struct(v); err != nil {
		return NewAppErrorWithDetails(ErrValidation, "Validation failed", ParseValidationErrors(err))
	}
	return nil
}

// ParseForm parses and validates form/urlencoded request
func (rb *RequestBuilder) ParseForm(v interface{}) error {
	if err := rb.request.ParseForm(); err != nil {
		return NewAppErrorWithErr(ErrInvalidFormat, "Failed to parse form", err)
	}

	values := rb.request.Form
	return rb.parseValues(values, v)
}

// ParseQuery parses and validates query parameters
func (rb *RequestBuilder) ParseQuery(v interface{}) error {
	values := rb.request.URL.Query()
	return rb.parseValues(values, v)
}

// parseValues parses url.Values into a struct using reflection
func (rb *RequestBuilder) parseValues(values url.Values, v interface{}) error {
	// Convert url.Values to map[string]interface{}
	data := make(map[string]interface{})
	for key, vals := range values {
		if len(vals) > 0 {
			data[key] = vals[0]
		}
	}

	// Marshal to JSON and unmarshal to target struct
	jsonData, err := json.Marshal(data)
	if err != nil {
		return NewAppErrorWithErr(ErrInvalidFormat, "Failed to parse query parameters", err)
	}

	if err := json.Unmarshal(jsonData, v); err != nil {
		return NewAppErrorWithErr(ErrInvalidFormat, "Failed to parse query parameters", err)
	}

	if err := validate.Struct(v); err != nil {
		return NewAppErrorWithDetails(ErrValidation, "Validation failed", ParseValidationErrors(err))
	}

	return nil
}

// GetHeader retrieves a header value from the request
func (rb *RequestBuilder) GetHeader(name string) string {
	return rb.request.Header.Get(name)
}

// GetParam retrieves a path parameter from mux vars
func (rb *RequestBuilder) GetParam(name string) string {
	vars := mux.Vars(rb.request)
	return vars[name]
}

// GetQuery retrieves a single query parameter with optional default
func (rb *RequestBuilder) GetQuery(name string, defaultValue string) string {
	if v := rb.request.URL.Query().Get(name); v != "" {
		return v
	}
	return defaultValue
}

// GetQueryInt retrieves a query parameter as int with default
func (rb *RequestBuilder) GetQueryInt(name string, defaultValue int) int {
	if v := rb.request.URL.Query().Get(name); v != "" {
		var val int
		if _, err := fmt.Sscanf(v, "%d", &val); err == nil {
			return val
		}
	}
	return defaultValue
}

// GetQueryBool retrieves a query parameter as bool with default
func (rb *RequestBuilder) GetQueryBool(name string, defaultValue bool) bool {
	if v := rb.request.URL.Query().Get(name); v != "" {
		switch v {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// GetPagination retrieves pagination parameters from query
func (rb *RequestBuilder) GetPagination() (page, limit int) {
	page = rb.GetQueryInt("page", 1)
	limit = rb.GetQueryInt("limit", 20)

	// Ensure valid values
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	return page, limit
}

// GetSort retrieves sort parameters from query
func (rb *RequestBuilder) GetSort() (sortBy, sortOrder string) {
	sortBy = rb.GetQuery("sort_by", "created_at")
	sortOrder = rb.GetQuery("sort_order", "desc")

	// Validate sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	return sortBy, sortOrder
}

// FileUpload represents an uploaded file
type FileUpload struct {
	File    *multipart.FileHeader `json:"-"`
	Name    string                `json:"name"`
	Size    int64                 `json:"size"`
	Type    string                `json:"type"`
	Content []byte                `json:"-"`
}

// ParseFile parses a single file upload from the request
// maxFileSize is in bytes (e.g., 10 << 20 = 10MB)
func (rb *RequestBuilder) ParseFile(fieldName string, maxFileSize int64) (*FileUpload, error) {
	if err := rb.request.ParseMultipartForm(maxFileSize); err != nil {
		return nil, NewAppErrorWithErr(ErrInvalidFormat, "Failed to parse multipart form", err)
	}

	file, header, err := rb.request.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, NewAppError(ErrMissingField, "File is required: "+fieldName)
		}
		return nil, NewAppErrorWithErr(ErrInvalidFormat, "Failed to retrieve file", err)
	}
	defer file.Close()

	// Check file size
	if header.Size > maxFileSize {
		return nil, NewAppErrorWithDetails(
			ErrInvalidFormat,
			fmt.Sprintf("File size exceeds limit of %d bytes", maxFileSize),
			map[string]int64{"max_size": maxFileSize, "received": header.Size},
		)
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, NewAppErrorWithErr(ErrInternal, "Failed to read file content", err)
	}

	return &FileUpload{
		File:    header,
		Name:    header.Filename,
		Size:    header.Size,
		Type:    header.Header.Get("Content-Type"),
		Content: content,
	}, nil
}

// ParseFiles parses multiple file uploads from the request
// maxFileSize is in bytes (e.g., 10 << 20 = 10MB)
func (rb *RequestBuilder) ParseFiles(fieldName string, maxFileSize, maxFiles int64) ([]*FileUpload, error) {
	if err := rb.request.ParseMultipartForm(maxFileSize * int64(maxFiles)); err != nil {
		return nil, NewAppErrorWithErr(ErrInvalidFormat, "Failed to parse multipart form", err)
	}

	form := rb.request.MultipartForm
	if form == nil || form.File[fieldName] == nil {
		return nil, NewAppError(ErrMissingField, "No files found for field: "+fieldName)
	}

	files := form.File[fieldName]
	if int64(len(files)) > maxFiles {
		return nil, NewAppErrorWithDetails(
			ErrInvalidFormat,
			fmt.Sprintf("Too many files: got %d, max %d", len(files), maxFiles),
			map[string]int64{"max_files": maxFiles, "received": int64(len(files))},
		)
	}

	uploads := make([]*FileUpload, 0, len(files))
	for _, fh := range files {
		// Check individual file size
		if fh.Size > maxFileSize {
			return nil, NewAppErrorWithDetails(
				ErrInvalidFormat,
				fmt.Sprintf("File %s exceeds size limit", fh.Filename),
				map[string]interface{}{"filename": fh.Filename, "max_size": maxFileSize},
			)
		}

		file, err := fh.Open()
		if err != nil {
			return nil, NewAppErrorWithErr(ErrInternal, "Failed to open file", err)
		}

		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return nil, NewAppErrorWithErr(ErrInternal, "Failed to read file content", err)
		}

		uploads = append(uploads, &FileUpload{
			File:    fh,
			Name:    fh.Filename,
			Size:    fh.Size,
			Type:    fh.Header.Get("Content-Type"),
			Content: content,
		})
	}

	return uploads, nil
}

// ValidateFileType checks if the file extension is allowed
func (rb *RequestBuilder) ValidateFileType(filename string, allowedExtensions []string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, allowed := range allowedExtensions {
		if ext == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

// ValidateFileMIMEType checks if the file MIME type is allowed
func (rb *RequestBuilder) ValidateFileMIMEType(contentType string, allowedMimeTypes []string) error {
	for _, allowed := range allowedMimeTypes {
		if allowed == "*/*" {
			return nil
		}
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "/*")
			if strings.HasPrefix(contentType, prefix) {
				return nil
			}
		}
		if contentType == allowed {
			return nil
		}
	}
	return NewAppErrorWithDetails(
		ErrInvalidFormat,
		fmt.Sprintf("File type '%s' is not allowed", contentType),
		map[string]interface{}{"content_type": contentType, "allowed_types": allowedMimeTypes},
	)
}

// SaveFile saves uploaded file to the specified path
func (rb *RequestBuilder) SaveFile(upload *FileUpload, dstPath string) error {
	if err := os.WriteFile(dstPath, upload.Content, 0644); err != nil {
		return NewAppErrorWithErr(ErrInternal, "Failed to save file", err)
	}
	return nil
}

// SaveFileToDir saves uploaded file to a directory with the original filename
func (rb *RequestBuilder) SaveFileToDir(upload *FileUpload, dir string) (string, error) {
	filename := filepath.Join(dir, upload.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", NewAppErrorWithErr(ErrInternal, "Failed to create directory", err)
	}
	if err := os.WriteFile(filename, upload.Content, 0644); err != nil {
		return "", NewAppErrorWithErr(ErrInternal, "Failed to save file", err)
	}
	return filename, nil
}

// SendFile sends a file as a downloadable attachment
func (rb *RequestBuilder) SendFile(w http.ResponseWriter, filePath string, filename string) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewAppError(ErrResourceNotFound, "File not found")
		}
		return NewAppErrorWithErr(ErrInternal, "Failed to open file", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return NewAppErrorWithErr(ErrInternal, "Failed to get file info", err)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	if _, err := io.Copy(w, file); err != nil {
		return NewAppErrorWithErr(ErrInternal, "Failed to send file", err)
	}
	return nil
}

// ServeFile serves a file inline (for viewing in browser)
func (rb *RequestBuilder) ServeFile(w http.ResponseWriter, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewAppError(ErrResourceNotFound, "File not found")
		}
		return NewAppErrorWithErr(ErrInternal, "Failed to open file", err)
	}
	defer file.Close()

	http.ServeContent(w, nil, filePath, time.Time{}, file)
	return nil
}

// GetClientIP extracts the client IP from the request
// It checks X-Forwarded-For, X-Real-IP, and falls back to RemoteAddr
func (rb *RequestBuilder) GetClientIP() string {
	// Check X-Forwarded-For header
	if xff := rb.request.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := rb.request.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(rb.request.RemoteAddr)
	if ip == "" {
		return rb.request.RemoteAddr
	}
	return ip
}

// GetUserAgent returns the User-Agent header
func (rb *RequestBuilder) GetUserAgent() string {
	return rb.request.Header.Get("User-Agent")
}

// IsJSONRequest checks if the request expects JSON response
func (rb *RequestBuilder) IsJSONRequest() bool {
	accept := rb.request.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

// IsAjaxRequest checks if the request is an AJAX request
func (rb *RequestBuilder) IsAjaxRequest() bool {
	return rb.request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// GetContentType returns the Content-Type header
func (rb *RequestBuilder) GetContentType() string {
	return rb.request.Header.Get("Content-Type")
}

// IsContentType checks if the request Content-Type matches the given type
func (rb *RequestBuilder) IsContentType(contentType string) bool {
	ct := rb.request.Header.Get(HeaderContentType)
	return strings.HasPrefix(ct, contentType)
}

// IsJSONContentType checks if the request Content-Type is JSON
func (rb *RequestBuilder) IsJSONContentType() bool {
	return rb.IsContentType(ContentTypeJSON)
}

// IsXMLContentType checks if the request Content-Type is XML
func (rb *RequestBuilder) IsXMLContentType() bool {
	return rb.IsContentType(ContentTypeXML)
}

// IsFormContentType checks if the request Content-Type is form data
func (rb *RequestBuilder) IsFormContentType() bool {
	return rb.IsContentType(ContentTypeForm) || rb.IsContentType(ContentTypeMultipartForm)
}

// AcceptsJSON checks if the client accepts JSON responses
func (rb *RequestBuilder) AcceptsJSON() bool {
	accept := rb.request.Header.Get(HeaderAccept)
	return strings.Contains(accept, ContentTypeJSON) || accept == "*/*" || accept == ""
}

// AcceptsXML checks if the client accepts XML responses
func (rb *RequestBuilder) AcceptsXML() bool {
	accept := rb.request.Header.Get(HeaderAccept)
	return strings.Contains(accept, ContentTypeXML) || accept == "*/*" || accept == ""
}

// AcceptsHTML checks if the client accepts HTML responses
func (rb *RequestBuilder) AcceptsHTML() bool {
	accept := rb.request.Header.Get(HeaderAccept)
	return strings.Contains(accept, ContentTypeHTML) || accept == "*/*" || accept == ""
}

// GetAcceptHeader returns the Accept header value
func (rb *RequestBuilder) GetAcceptHeader() string {
	return rb.request.Header.Get(HeaderAccept)
}

// GetAuthorizationHeader returns the Authorization header value
func (rb *RequestBuilder) GetAuthorizationHeader() string {
	return rb.request.Header.Get(HeaderAuthorization)
}

// GetBearerToken extracts the bearer token from the Authorization header
// Returns empty string if not a bearer token or header is missing
func (rb *RequestBuilder) GetBearerToken() string {
	auth := rb.GetAuthorizationHeader()
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return auth[len(prefix):]
	}
	return ""
}

// GetFormValue gets a form value with optional default
func (rb *RequestBuilder) GetFormValue(key, defaultValue string) string {
	if v := rb.request.FormValue(key); v != "" {
		return v
	}
	return defaultValue
}

// GetFormInt gets a form value as int with default
func (rb *RequestBuilder) GetFormInt(key string, defaultValue int) int {
	if v := rb.request.FormValue(key); v != "" {
		var val int
		if _, err := fmt.Sscanf(v, "%d", &val); err == nil {
			return val
		}
	}
	return defaultValue
}

// GetFormBool gets a form value as bool with default
func (rb *RequestBuilder) GetFormBool(key string, defaultValue bool) bool {
	if v := rb.request.FormValue(key); v != "" {
		switch v {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// GetFormFloat gets a form value as float64 with default
func (rb *RequestBuilder) GetFormFloat(key string, defaultValue float64) float64 {
	if v := rb.request.FormValue(key); v != "" {
		var val float64
		if _, err := fmt.Sscanf(v, "%f", &val); err == nil {
			return val
		}
	}
	return defaultValue
}

// HasFormValue checks if a form value exists and is not empty
func (rb *RequestBuilder) HasFormValue(key string) bool {
	return rb.request.FormValue(key) != ""
}

// GetFormValues gets all form values for a key
func (rb *RequestBuilder) GetFormValues(key string) []string {
	if rb.request.Form == nil {
		return nil
	}
	return rb.request.Form[key]
}
