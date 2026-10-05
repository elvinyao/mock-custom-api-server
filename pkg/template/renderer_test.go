package template

import "testing"

func TestRenderer_SimpleLiteralSelectorNames(t *testing.T) {
	for _, name := range []string{"user-id", "user.id", "使用者", "123", "user name", "a{b}c", "user/id:kind"} {
		t.Run(name, func(t *testing.T) {
			renderer, err := NewRenderer(map[string]string{name: "value"}, "simple")
			if err != nil {
				t.Fatal(err)
			}
			content := []byte("before {{." + name + "}} after")
			if err := Validate(content, "simple"); err != nil {
				t.Fatalf("literal selector name rejected: %v", err)
			}
			got, err := renderer.Render(content)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "before value after" {
				t.Fatalf("rendered %q", got)
			}
		})
	}
}

func TestRenderer_SimpleRejectsMalformedDelimiters(t *testing.T) {
	for _, content := range []string{"{{.}}", "{{.name", "{{name}}", "{{.outer{{.inner}}"} {
		t.Run(content, func(t *testing.T) {
			if err := Validate([]byte(content), "simple"); err == nil {
				t.Fatal("malformed template accepted")
			}
		})
	}
}

func TestRenderer_SimplePreservesLiteralClosingBraces(t *testing.T) {
	for _, tc := range []struct{ content, want string }{
		{`{"data":{"id":"{{.id}}"}}`, `{"data":{"id":"123"}}`},
		{"before }} after", "before }} after"},
		{"literal }} {{.id}} }} end", "literal }} 123 }} end"},
	} {
		t.Run(tc.content, func(t *testing.T) {
			if err := Validate([]byte(tc.content), "simple"); err != nil {
				t.Fatalf("literal closing braces rejected: %v", err)
			}
			renderer, err := NewRenderer(map[string]string{"id": "123"}, "simple")
			if err != nil {
				t.Fatal(err)
			}
			got, err := renderer.Render([]byte(tc.content))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("rendered %q, want %q", got, tc.want)
			}
		})
	}
}
