package TraefikCookiePathReplacement

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

const setCookieHeader string = "Set-Cookie"

type Config struct {
	Replacements []ReplacementConfig `json:"replacements,omitempty" toml:"replacements,omitempty" yaml:"replacements,omitempty"`
}

type ReplacementConfig struct {
	Name        string `json:"name_regex,omitempty" toml:"name_regex,omitempty" yaml:"name_regex,omitempty"`    //Regex to match the cookie name, if empty it will match all cookies
	Original    string `json:"original,omitempty" toml:"original,omitempty" yaml:"original,omitempty"`          //Regex to match the original path
	Replacement string `json:"replacement,omitempty" toml:"replacement,omitempty" yaml:"replacement,omitempty"` //Replacement path, can use {{group_name}} to reference named capture groups from the original regex
}

func (rc *ReplacementConfig) compile() (compiledReplacement, error) {
	var name *regexp.Regexp
	var err error
	if rc.Name != "" {
		name, err = regexp.Compile("^" + rc.Name + "$")
		if err != nil {
			return compiledReplacement{}, err
		}
	}

	original, err := regexp.Compile("^" + rc.Original + "$")
	if err != nil {
		return compiledReplacement{}, err
	}

	return compiledReplacement{
		name:        name,
		original:    original,
		replacement: rc.Replacement,
	}, nil
}

// CreateConfig creates and initializes the plugin configuration.
func CreateConfig() *Config {
	return &Config{}
}

type compiledReplacement struct {
	name        *regexp.Regexp
	original    *regexp.Regexp
	replacement string
}

type rewriteBody struct {
	name                 string
	next                 http.Handler
	config               *Config
	compiledReplacements []compiledReplacement
}

func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {

	crs := make([]compiledReplacement, len(config.Replacements))
	for i, replacement := range config.Replacements {
		cr, err := replacement.compile()
		if err != nil {
			return nil, err
		}
		crs[i] = cr
	}

	return &rewriteBody{
		name:                 name,
		next:                 next,
		config:               config,
		compiledReplacements: crs,
	}, nil
}

func (r *rewriteBody) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	wrappedWriter := &responseWriter{
		writer: rw,
		config: r.compiledReplacements,
	}

	r.next.ServeHTTP(wrappedWriter, req)
}

type responseWriter struct {
	writer http.ResponseWriter
	config []compiledReplacement
}

func (r *responseWriter) Header() http.Header {
	return r.writer.Header()
}

func (r *responseWriter) Write(bytes []byte) (int, error) {
	return r.writer.Write(bytes)
}

func (r *responseWriter) WriteHeader(statusCode int) {
	headers := r.writer.Header()
	req := http.Response{Header: headers}
	cookies := req.Cookies()

	r.writer.Header().Del(setCookieHeader)

	for _, cookie := range cookies {

		for _, replacement := range r.config {
			if replacement.name != nil && !replacement.name.MatchString(cookie.Name) {
				continue
			}
			if replacement.original.MatchString(cookie.Path) {
				replacementPath := replacement.replacement
				if strings.Contains(replacement.replacement, "{{") { //No point in doing this if there are no named capture groups
					matchGroups := replacement.original.FindStringSubmatch(cookie.Path)
					for i, name := range replacement.original.SubexpNames() {
						if i != 0 && name != "" {
							replacementPath = strings.ReplaceAll(replacementPath, "{{"+name+"}}", matchGroups[i])
						}
					}
				}
				cookie.Path = replacementPath
			}
		}

		http.SetCookie(r, cookie)
	}

	r.writer.WriteHeader(statusCode)
}
