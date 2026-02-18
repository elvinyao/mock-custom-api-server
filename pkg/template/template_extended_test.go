package template

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// ── ReplaceVariablesWithEngine – go engine ─────────────────────────────────────

func TestReplaceVariablesWithEngine_GoEngine_ValueSubstitution(t *testing.T) {
	content := []byte(`{"id":"{{index . "uid"}}"}`)
	values := map[string]string{"uid": "u-42"}
	result := ReplaceVariablesWithEngine(content, values, "go")
	if string(result) != `{"id":"u-42"}` {
		t.Errorf("go engine result = %q, want {\"id\":\"u-42\"}", string(result))
	}
}

func TestReplaceVariablesWithEngine_GoEngine_FallbackOnParseError(t *testing.T) {
	// Invalid go template → should fall back to simple engine
	content := []byte(`{"val":"{{.myval}}"}`)
	values := map[string]string{"myval": "hello"}
	result := ReplaceVariablesWithEngine(content, values, "go")
	if string(result) != `{"val":"hello"}` {
		t.Errorf("fallback result = %q, want {\"val\":\"hello\"}", string(result))
	}
}

func TestReplaceVariablesWithEngine_SimpleEngine(t *testing.T) {
	content := []byte(`hello {{.name}}`)
	values := map[string]string{"name": "world"}
	result := ReplaceVariablesWithEngine(content, values, "simple")
	if string(result) != "hello world" {
		t.Errorf("simple engine = %q, want \"hello world\"", string(result))
	}
}

func TestReplaceVariablesWithEngine_DefaultEngine(t *testing.T) {
	content := []byte(`hi {{.x}}`)
	values := map[string]string{"x": "123"}
	result := ReplaceVariablesWithEngine(content, values, "")
	if string(result) != "hi 123" {
		t.Errorf("default engine = %q, want \"hi 123\"", string(result))
	}
}

// ── buildFuncMap – individual functions ───────────────────────────────────────

func TestFuncMap_RandomInt(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["randomInt"].(func(int, int) int)

	// max <= min: returns min
	if v := fn(5, 5); v != 5 {
		t.Errorf("randomInt(5,5) = %d, want 5", v)
	}
	if v := fn(5, 3); v != 5 {
		t.Errorf("randomInt(5,3) = %d, want 5 (max<=min fallback)", v)
	}
	// In range
	for i := 0; i < 50; i++ {
		v := fn(0, 10)
		if v < 0 || v >= 10 {
			t.Errorf("randomInt(0,10) = %d out of range", v)
		}
	}
}

func TestFuncMap_RandomFloat(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["randomFloat"].(func(float64, float64) float64)

	if v := fn(5.0, 5.0); v != 5.0 {
		t.Errorf("randomFloat(5,5) = %f, want 5.0", v)
	}
	for i := 0; i < 50; i++ {
		v := fn(0.0, 1.0)
		if v < 0.0 || v >= 1.0 {
			t.Errorf("randomFloat(0,1) = %f out of range", v)
		}
	}
}

func TestFuncMap_RandomChoice(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["randomChoice"].(func(...string) string)

	if v := fn(); v != "" {
		t.Errorf("randomChoice() = %q, want empty string", v)
	}
	v := fn("a", "b", "c")
	if v != "a" && v != "b" && v != "c" {
		t.Errorf("randomChoice got unexpected value %q", v)
	}
}

func TestFuncMap_TimestampMs(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["timestampMs"].(func() int64)
	v := fn()
	if v <= 0 {
		t.Errorf("timestampMs() = %d, want > 0", v)
	}
}

func TestFuncMap_Timestamp(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["timestamp"].(func() string)
	v := fn()
	if v == "" {
		t.Error("timestamp() returned empty string")
	}
}

func TestFuncMap_UUID(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["uuid"].(func() string)
	v := fn()
	// UUID format: 8-4-4-4-12 hex chars
	if len(v) != 36 {
		t.Errorf("uuid() length = %d, want 36", len(v))
	}
}

func TestFuncMap_Base64Encode(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["base64Encode"].(func(string) string)
	v := fn("hello")
	expected := base64.StdEncoding.EncodeToString([]byte("hello"))
	if v != expected {
		t.Errorf("base64Encode(\"hello\") = %q, want %q", v, expected)
	}
}

func TestFuncMap_JsonEscape(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["jsonEscape"].(func(string) string)
	v := fn(`say "hi"`)
	if !strings.Contains(v, `\"`) {
		t.Errorf("jsonEscape: expected escaped quotes, got %q", v)
	}
}

func TestFuncMap_ArithmeticOps(t *testing.T) {
	fm := buildFuncMap()
	add := fm["add"].(func(int, int) int)
	sub := fm["sub"].(func(int, int) int)
	mul := fm["mul"].(func(int, int) int)
	div := fm["div"].(func(int, int) int)

	if add(3, 4) != 7 {
		t.Error("add(3,4) != 7")
	}
	if sub(10, 3) != 7 {
		t.Error("sub(10,3) != 7")
	}
	if mul(3, 4) != 12 {
		t.Error("mul(3,4) != 12")
	}
	if div(10, 2) != 5 {
		t.Error("div(10,2) != 5")
	}
	// Division by zero → 0
	if div(10, 0) != 0 {
		t.Error("div(10,0) != 0")
	}
}

func TestFuncMap_Env(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["env"].(func(string) string)
	os.Setenv("TEST_TMPL_KEY", "testval")
	defer os.Unsetenv("TEST_TMPL_KEY")
	if v := fn("TEST_TMPL_KEY"); v != "testval" {
		t.Errorf("env(\"TEST_TMPL_KEY\") = %q, want testval", v)
	}
	if v := fn("NONEXISTENT_9876"); v != "" {
		t.Errorf("env(nonexistent) = %q, want empty", v)
	}
}

func TestFuncMap_StringOps(t *testing.T) {
	fm := buildFuncMap()
	upper := fm["upper"].(func(string) string)
	lower := fm["lower"].(func(string) string)
	trim := fm["trim"].(func(string) string)
	replace := fm["replace"].(func(string, string, string) string)

	if upper("hello") != "HELLO" {
		t.Error("upper(\"hello\") != HELLO")
	}
	if lower("WORLD") != "world" {
		t.Error("lower(\"WORLD\") != world")
	}
	if trim("  hi  ") != "hi" {
		t.Error("trim(\"  hi  \") != hi")
	}
	if replace("aXbXc", "X", "-") != "a-b-c" {
		t.Error("replace failed")
	}
}

func TestFuncMap_Sprintf(t *testing.T) {
	fm := buildFuncMap()
	fn := fm["sprintf"].(func(string, ...interface{}) string)
	v := fn("hello %s %d", "world", 42)
	if v != "hello world 42" {
		t.Errorf("sprintf = %q, want \"hello world 42\"", v)
	}
}

// ── go template with funcmap ───────────────────────────────────────────────────

func TestGoTemplate_WithRandomInt(t *testing.T) {
	content := []byte(`{{ randomInt 0 100 }}`)
	result := ReplaceVariablesWithEngine(content, nil, "go")
	r := strings.TrimSpace(string(result))
	// Should be a number string 0-99
	if r == "" {
		t.Error("go template randomInt produced empty output")
	}
}

func TestGoTemplate_WithBase64(t *testing.T) {
	content := []byte(`{{ base64Encode "test" }}`)
	result := ReplaceVariablesWithEngine(content, nil, "go")
	expected := base64.StdEncoding.EncodeToString([]byte("test"))
	if strings.TrimSpace(string(result)) != expected {
		t.Errorf("go template base64Encode = %q, want %q", string(result), expected)
	}
}

// ── cleanUnmatchedPlaceholders ─────────────────────────────────────────────────

func TestCleanUnmatchedPlaceholders_RemovesPlaceholders(t *testing.T) {
	input := `{"id":"{{.missing_key}}","name":"john"}`
	result := cleanUnmatchedPlaceholders(input)
	if strings.Contains(result, "{{.") {
		t.Errorf("expected no placeholders remaining, got: %q", result)
	}
	if !strings.Contains(result, `"name":"john"`) {
		t.Errorf("non-placeholder content removed: %q", result)
	}
}

func TestCleanUnmatchedPlaceholders_NothingToRemove(t *testing.T) {
	input := `{"id":"123"}`
	result := cleanUnmatchedPlaceholders(input)
	if result != input {
		t.Errorf("expected unchanged, got %q", result)
	}
}

func TestCleanUnmatchedPlaceholders_MultipleRemoved(t *testing.T) {
	input := `{{.a}} {{.b}} text {{.c}}`
	result := cleanUnmatchedPlaceholders(input)
	if strings.Contains(result, "{{.") {
		t.Errorf("still has placeholders: %q", result)
	}
	if !strings.Contains(result, "text") {
		t.Errorf("literal text was removed: %q", result)
	}
}
