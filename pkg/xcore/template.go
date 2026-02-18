package xcore

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// TemplateConfig holds the configuration for the template engine
type TemplateConfig struct {
	// BaseDir is the base directory for template files
	BaseDir string `mapstructure:"base_dir"`
	// Extension is the default extension for template files (default: ".html")
	Extension string `mapstructure:"extension"`
	// Reload enables template reloading on each request (useful for development)
	Reload bool `mapstructure:"reload"`
	// Delims sets custom template delimiters
	Delims *TemplateDelims `mapstructure:"delims"`
}

// TemplateDelims holds custom template delimiters
type TemplateDelims struct {
	Left  string `mapstructure:"left"`
	Right string `mapstructure:"right"`
}

// TemplateData represents the data passed to templates
type TemplateData struct {
	Title string
	Data  map[string]interface{}
	// Blocks that can be overridden by child templates
	CSS     template.HTML
	Content template.HTML
	JS      template.HTML
}

// TemplateEngine handles template rendering
type TemplateEngine struct {
	templates *template.Template
	config    *TemplateConfig
	logger    *zerolog.Logger
}

// NewTemplateEngine creates a new template engine with the given configuration
func NewTemplateEngine(cfg *TemplateConfig, logger *zerolog.Logger) (*TemplateEngine, error) {
	if cfg.Extension == "" {
		cfg.Extension = ".html"
	}

	te := &TemplateEngine{
		config: cfg,
		logger: logger,
	}

	// Load templates if base directory is specified
	if cfg.BaseDir != "" {
		if err := te.LoadTemplates(); err != nil {
			return nil, err
		}
	}

	return te, nil
}

// LoadTemplates loads all templates from the configured base directory
func (te *TemplateEngine) LoadTemplates() error {
	if te.config.BaseDir == "" {
		return nil
	}

	pattern := filepath.Join(te.config.BaseDir, "**/*"+te.config.Extension)
	te.logger.Debug().Str("pattern", pattern).Msg("Loading templates")

	var tmpl *template.Template
	var err error

	if te.config.Delims != nil {
		tmpl = template.New("").Delims(te.config.Delims.Left, te.config.Delims.Right)
	} else {
		tmpl = template.New("")
	}

	tmpl = tmpl.Funcs(TemplateFuncMap())
	tmpl, err = tmpl.ParseGlob(pattern)
	if err != nil {
		return err
	}

	te.templates = tmpl
	te.logger.Info().Str("path", te.config.BaseDir).Msg("Templates loaded successfully")
	return nil
}

// Reload reloads all templates (useful for development)
func (te *TemplateEngine) Reload() error {
	return te.LoadTemplates()
}

// Render renders a template with the given data
func (te *TemplateEngine) Render(w http.ResponseWriter, r *http.Request, templateName string, data *TemplateData) error {
	// Reload templates if enabled (for development)
	if te.config.Reload {
		if err := te.Reload(); err != nil {
			te.logger.Error().Err(err).Msg("Failed to reload templates")
			return err
		}
	}

	if te.templates == nil {
		te.logger.Error().Msg("No templates loaded")
		http.Error(w, "Templates not configured", http.StatusInternalServerError)
		return nil
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute template
	err := te.templates.ExecuteTemplate(w, templateName, data)
	if err != nil {
		te.logger.Error().Err(err).Str("template", templateName).Msg("Template execution failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return nil
}

// MustRender is like Render but panics on error
func (te *TemplateEngine) MustRender(w http.ResponseWriter, r *http.Request, templateName string, data *TemplateData) {
	if err := te.Render(w, r, templateName, data); err != nil {
		panic(err)
	}
}

// TemplateMiddleware creates middleware that reloads templates in development mode
func (te *TemplateEngine) TemplateMiddleware() mux.MiddlewareFunc {
	if !te.config.Reload {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reload templates before each request
			if err := te.Reload(); err != nil {
				te.logger.Error().Err(err).Msg("Failed to reload templates")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NewTemplateData creates a new TemplateData with default values
func NewTemplateData() *TemplateData {
	return &TemplateData{
		Data: make(map[string]interface{}),
	}
}

// WithTitle sets the title
func (td *TemplateData) WithTitle(title string) *TemplateData {
	td.Title = title
	return td
}

// WithCSS sets the CSS block content
func (td *TemplateData) WithCSS(css template.HTML) *TemplateData {
	td.CSS = css
	return td
}

// WithContent sets the content block
func (td *TemplateData) WithContent(content template.HTML) *TemplateData {
	td.Content = content
	return td
}

// WithContentString sets the content from a string
func (td *TemplateData) WithContentString(content string) *TemplateData {
	td.Content = template.HTML(content)
	return td
}

// WithJS sets the JS block content
func (td *TemplateData) WithJS(js template.HTML) *TemplateData {
	td.JS = js
	return td
}

// WithData sets custom data
func (td *TemplateData) WithData(key string, value interface{}) *TemplateData {
	td.Data[key] = value
	return td
}

// WithDataMap sets multiple custom data values
func (td *TemplateData) WithDataMap(data map[string]interface{}) *TemplateData {
	for k, v := range data {
		td.Data[k] = v
	}
	return td
}

// TemplateFuncMap returns common template functions
func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
		"trim":  strings.TrimSpace,
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"safeJS": func(s string) template.JS {
			return template.JS(s)
		},
		"safeCSS": func(s string) template.CSS {
			return template.CSS(s)
		},
		"safeURL": func(s string) template.URL {
			return template.URL(s)
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"hasPrefix": func(s, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
		"hasSuffix": func(s, suffix string) bool {
			return strings.HasSuffix(s, suffix)
		},
		"join": func(sep string, elems []string) string {
			return strings.Join(elems, sep)
		},
		"split": func(sep, s string) []string {
			return strings.Split(s, sep)
		},
		"repeat": func(count int, s string) string {
			return strings.Repeat(s, count)
		},
		"now": time.Now,
		"formatTime": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"ne": func(a, b interface{}) bool {
			return a != b
		},
		"lt": func(a, b int) bool {
			return a < b
		},
		"lte": func(a, b int) bool {
			return a <= b
		},
		"gt": func(a, b int) bool {
			return a > b
		},
		"gte": func(a, b int) bool {
			return a >= b
		},
		"or": func(a, b bool) bool {
			return a || b
		},
		"and": func(a, b bool) bool {
			return a && b
		},
		"not": func(a bool) bool {
			return !a
		},
		"default": func(val, def interface{}) interface{} {
			if val == nil || val == "" || val == false {
				return def
			}
			return val
		},
		"len": func(s interface{}) int {
			switch v := s.(type) {
			case string:
				return len(v)
			case []interface{}:
				return len(v)
			case map[string]interface{}:
				return len(v)
			default:
				return 0
			}
		},
		"index": func(i int, s []string) string {
			if i < 0 || i >= len(s) {
				return ""
			}
			return s[i]
		},
	}
}
