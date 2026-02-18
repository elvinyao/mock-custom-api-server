package proxy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mock-api-server/config"

	"gopkg.in/yaml.v3"
)

// StubWriter saves recorded proxy interactions as YAML endpoint stubs
type StubWriter struct{}

// WriteStub saves a recorded interaction as a ready-to-use YAML endpoint + JSON response file
func (sw StubWriter) WriteStub(recordDir string, req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	if err := os.MkdirAll(recordDir, 0755); err != nil {
		return
	}

	// Generate file-safe names
	safePath := strings.ReplaceAll(strings.Trim(req.URL.Path, "/"), "/", "_")
	if safePath == "" {
		safePath = "root"
	}
	ts := time.Now().Format("20060102_150405")
	baseName := fmt.Sprintf("%s_%s_%s", strings.ToLower(req.Method), safePath, ts)

	// Write response body as JSON file
	jsonFile := filepath.Join(recordDir, baseName+".json")
	if err := os.WriteFile(jsonFile, respBody, 0644); err != nil {
		return
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
		return
	}

	yamlFile := filepath.Join(recordDir, baseName+".yaml")
	_ = os.WriteFile(yamlFile, data, 0644)
}
