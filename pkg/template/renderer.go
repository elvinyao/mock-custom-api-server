package template

import (
	"bytes"
	"fmt"
	"strings"
	gotemplate "text/template"
)

type simplePlaceholder struct {
	start, end int
	name       string
}

// Selector names are literal map keys, not Go identifiers. Only the template
// delimiters and the leading dot have syntax in the simple engine.
func parseSimple(content string) ([]simplePlaceholder, error) {
	var placeholders []simplePlaceholder
	invalid := fmt.Errorf("invalid simple template placeholder; expected {{.name}}")
	for cursor := 0; cursor < len(content); {
		start := strings.Index(content[cursor:], "{{")
		if start == -1 {
			break
		}
		start += cursor
		close := strings.Index(content[start+2:], "}}")
		if close == -1 {
			return nil, invalid
		}
		end := start + 2 + close
		expression := content[start+2 : end]
		if !strings.HasPrefix(expression, ".") || len(expression) == 1 || strings.Contains(expression, "{{") {
			return nil, invalid
		}
		placeholders = append(placeholders, simplePlaceholder{start: start, end: end + 2, name: expression[1:]})
		cursor = end + 2
	}
	return placeholders, nil
}

// Renderer holds one response's values, including shared built-in variables.
type Renderer struct {
	engine string
	values map[string]string
}

// NewRenderer creates a strict renderer. Reuse it for a response's body and headers.
func NewRenderer(values map[string]string, engine string) (*Renderer, error) {
	engine = strings.ToLower(engine)
	if engine == "" {
		engine = "simple"
	}
	if engine != "simple" && engine != "go" {
		return nil, fmt.Errorf("unknown template engine %q", engine)
	}
	data := getBuiltinVariables()
	for k, v := range values {
		data[k] = v
	}
	return &Renderer{engine: engine, values: data}, nil
}

// Validate checks syntax without evaluating request-dependent expressions.
func Validate(content []byte, engine string) error {
	switch strings.ToLower(engine) {
	case "", "simple":
		_, err := parseSimple(string(content))
		return err
	case "go":
		_, err := gotemplate.New("response").Funcs(buildFuncMap()).Parse(string(content))
		return err
	default:
		return fmt.Errorf("unknown template engine %q", engine)
	}
}

// Render returns parsing and execution errors instead of falling back to another engine.
func (r *Renderer) Render(content []byte) ([]byte, error) {
	if r.engine == "simple" {
		placeholders, err := parseSimple(string(content))
		if err != nil {
			return nil, err
		}
		var result bytes.Buffer
		cursor := 0
		for _, placeholder := range placeholders {
			value, ok := r.values[placeholder.name]
			if !ok {
				return nil, fmt.Errorf("missing template variable %q", placeholder.name)
			}
			result.Write(content[cursor:placeholder.start])
			result.WriteString(value)
			cursor = placeholder.end
		}
		result.Write(content[cursor:])
		return result.Bytes(), nil
	}
	if err := Validate(content, r.engine); err != nil {
		return nil, err
	}
	funcs := buildFuncMap()
	funcs["uuid"] = func() string { return r.values["uuid"] }
	funcs["timestamp"] = func() string { return r.values["timestamp"] }
	tmpl, err := gotemplate.New("response").Option("missingkey=error").Funcs(funcs).Parse(string(content))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, r.values); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
