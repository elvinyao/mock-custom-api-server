package proxy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"mock-api-server/config"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// StubWriter saves recorded proxy interactions as YAML endpoint stubs
type StubWriter struct{}

// WriteStub saves a recorded interaction as a ready-to-use YAML endpoint + JSON response file
func (sw StubWriter) WriteStub(recordDir string, req *http.Request, resp *http.Response, reqBody, respBody []byte) error {
	if err := os.MkdirAll(recordDir, 0755); err != nil {
		return fmt.Errorf("create recording directory: %w", err)
	}

	// Generate file-safe names
	safePath := strings.Map(func(r rune) rune {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
			return r
		}
		return '_'
	}, strings.Trim(req.URL.Path, "/"))
	if len(safePath) > 80 {
		safePath = safePath[:80]
	}
	if safePath == "" {
		safePath = "root"
	}
	ts := time.Now().Format("20060102_150405")
	baseName := fmt.Sprintf("%s_%s_%s_%s", strings.ToLower(req.Method), safePath, ts, uuid.NewString())

	// Write response body as JSON file
	jsonFile := filepath.Join(recordDir, baseName+".json")
	if err := writeRecordingFile(jsonFile, respBody); err != nil {
		return fmt.Errorf("write recorded body: %w", err)
	}

	// Build endpoint stub
	ep := config.Endpoint{
		Path:        req.URL.Path,
		Method:      req.Method,
		Description: fmt.Sprintf("Recorded from %s %s at %s", req.Method, req.URL.Path, ts),
		Default: config.ResponseConfig{
			ResponseFile: jsonFile,
			StatusCode:   resp.StatusCode,
			ContentType:  resp.Header.Get("Content-Type"),
		},
	}

	data, err := yaml.Marshal([]config.Endpoint{ep})
	if err != nil {
		_ = os.Remove(jsonFile)
		return fmt.Errorf("encode recorded endpoint: %w", err)
	}

	yamlFile := filepath.Join(recordDir, baseName+".yaml")
	if err := writeRecordingFile(yamlFile, data); err != nil {
		_ = os.Remove(jsonFile)
		return fmt.Errorf("write recorded endpoint: %w", err)
	}
	return nil
}

// Publish the YAML last so any visible stub refers to a complete response file.
func writeRecordingFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".recording-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
