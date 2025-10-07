package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestLoadAllowlist(t *testing.T) {
	allowlist, err := loadAllowlist()
	if err != nil {
		t.Fatalf("Failed to load allowlist: %v", err)
	}

	if len(allowlist) == 0 {
		t.Error("Allowlist is empty")
	}

	// Check that the embedded allowlist contains expected entries
	expectedEntries := map[string]bool{
		"example.com":        true,
		"example.org":        true,
		"www.google.com:443": true,
		"api.github.com:443": true,
	}

	for _, entry := range allowlist {
		if expectedEntries[entry] {
			delete(expectedEntries, entry)
		}
	}

	if len(expectedEntries) > 0 {
		t.Errorf("Missing expected allowlist entries: %v", expectedEntries)
	}
}

func TestLoadAllowlistInvalidYAML(t *testing.T) {
	// Save original
	originalYAML := allowlistYAML

	// Temporarily replace with invalid YAML
	allowlistYAML = []byte("invalid: yaml: content: [")

	_, err := loadAllowlist()
	if err == nil {
		t.Error("Expected error when parsing invalid YAML")
	}

	// Restore
	allowlistYAML = originalYAML
}

func TestNewProxyServer(t *testing.T) {
	logger := NewLogger(os.Stdout)
	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	if proxy.listen != "localhost:8080" {
		t.Errorf("Expected listen localhost:8080, got %s", proxy.listen)
	}

	if proxy.logger == nil {
		t.Error("Logger is nil")
	}

	if len(proxy.allowlist) == 0 {
		t.Error("Allowlist is empty")
	}

	// Check discovery mode (should be false by default)
	if DiscoveryMode == "true" {
		if !proxy.discoveryMode {
			t.Error("Discovery mode should be enabled")
		}
	} else {
		if proxy.discoveryMode {
			t.Error("Discovery mode should be disabled")
		}
	}
}

func TestIsAllowed(t *testing.T) {
	logger := NewLogger(os.Stdout)
	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	tests := []struct {
		name     string
		hostPort string
		allowed  bool
	}{
		{
			name:     "exact match with port",
			hostPort: "api.github.com:443",
			allowed:  true,
		},
		{
			name:     "hostname only - any port",
			hostPort: "example.com:443",
			allowed:  true,
		},
		{
			name:     "hostname only - different port",
			hostPort: "example.com:8080",
			allowed:  true,
		},
		{
			name:     "blocked - not in allowlist",
			hostPort: "evil.com:443",
			allowed:  false,
		},
		{
			name:     "blocked - different host",
			hostPort: "notallowed.com:443",
			allowed:  false,
		},
		{
			name:     "google with exact port match",
			hostPort: "www.google.com:443",
			allowed:  true,
		},
		{
			name:     "google with different port - blocked",
			hostPort: "www.google.com:80",
			allowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proxy.isAllowed(tt.hostPort)
			if result != tt.allowed {
				t.Errorf("isAllowed(%s) = %v, want %v", tt.hostPort, result, tt.allowed)
			}
		})
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	// Test Info
	logger.Info("test_event", "test message")
	output := buf.String()
	buf.Reset()

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if entry.Level != LogLevelInfo {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
	if entry.Event != "test_event" {
		t.Errorf("Expected event test_event, got %s", entry.Event)
	}
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", entry.Message)
	}

	// Test ConnectionAttempt
	logger.ConnectionAttempt("example.com:443", "allowed", nil)
	output = buf.String()

	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if entry.Event != "connection_attempt" {
		t.Errorf("Expected event connection_attempt, got %s", entry.Event)
	}
	if entry.Destination != "example.com:443" {
		t.Errorf("Expected destination example.com:443, got %s", entry.Destination)
	}
	if entry.Action != "allowed" {
		t.Errorf("Expected action allowed, got %s", entry.Action)
	}
}

func TestHandleConnectMethodNotAllowed(t *testing.T) {
	logger := NewLogger(os.Stdout)
	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	// Test non-CONNECT method
	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	proxy.handleConnect(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleConnectBlocked(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	// Make sure we're not in discovery mode for this test
	proxy.discoveryMode = false

	// Test blocked destination
	req := httptest.NewRequest("CONNECT", "http://blocked.com:443", nil)
	req.Host = "blocked.com:443"
	w := httptest.NewRecorder()

	proxy.handleConnect(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	// Check log
	output := buf.String()
	if !strings.Contains(output, "blocked") {
		t.Errorf("Expected log to contain 'blocked', got: %s", output)
	}
}

func TestHandleConnectAllowed(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	proxy.discoveryMode = false

	// Test allowed destination (example.com is in allowlist)
	req := httptest.NewRequest("CONNECT", "http://example.com:443", nil)
	req.Host = "example.com:443"
	w := httptest.NewRecorder()

	proxy.handleConnect(w, req)

	// The connection will fail because we can't actually hijack in tests,
	// but we should at least get past the allowlist check
	// Check that we logged the allowed connection
	output := buf.String()
	if !strings.Contains(output, "allowed") {
		t.Errorf("Expected log to contain 'allowed', got: %s", output)
	}
}

func TestDiscoveryMode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	// Force discovery mode for this test
	proxy.discoveryMode = true

	// Test that even blocked destinations are allowed in discovery mode
	req := httptest.NewRequest("CONNECT", "http://should-be-blocked.com:443", nil)
	req.Host = "should-be-blocked.com:443"
	w := httptest.NewRecorder()

	proxy.handleConnect(w, req)

	// Should not return forbidden in discovery mode
	if w.Code == http.StatusForbidden {
		t.Error("Discovery mode should not block any connections")
	}

	// Check log for discovery action
	output := buf.String()
	if !strings.Contains(output, "allowed_discovery") {
		t.Errorf("Expected log to contain 'allowed_discovery', got: %s", output)
	}
}

func TestConfigStructure(t *testing.T) {
	yamlContent := `allowlist:
  - example.com
  - test.com:443
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if len(config.Allowlist) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(config.Allowlist))
	}

	if config.Allowlist[0] != "example.com" {
		t.Errorf("Expected first entry to be example.com, got %s", config.Allowlist[0])
	}
}

func TestProxyServerIntegration(t *testing.T) {
	// Create a test server to act as destination
	destServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer destServer.Close()

	// Parse destination host:port
	destHost := strings.TrimPrefix(destServer.URL, "http://")

	// Create temporary allowlist with test server
	tempAllowlist := []byte(`allowlist:
  - ` + destHost + `
`)
	originalAllowlist := allowlistYAML
	allowlistYAML = tempAllowlist

	logger := NewLogger(os.Stdout)
	proxy, err := NewProxyServer("localhost:0", logger) // Use port 0 for random port
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	// Check that test server is in allowlist
	if !proxy.isAllowed(destHost) {
		t.Errorf("Test server %s should be in allowlist", destHost)
	}

	// Restore original allowlist
	allowlistYAML = originalAllowlist
}

func TestLogEntryFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	entry := LogEntry{
		Level:       LogLevelInfo,
		Event:       "test_event",
		Destination: "example.com:443",
		Action:      "allowed",
		Message:     "test message",
	}

	logger.Log(entry)

	var parsed LogEntry
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Check timestamp was added
	if parsed.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}

	// Verify timestamp format
	if _, err := time.Parse(time.RFC3339, parsed.Timestamp); err != nil {
		t.Errorf("Invalid timestamp format: %s", parsed.Timestamp)
	}

	// Check all fields
	if parsed.Level != LogLevelInfo {
		t.Errorf("Expected level INFO, got %s", parsed.Level)
	}
	if parsed.Event != "test_event" {
		t.Errorf("Expected event test_event, got %s", parsed.Event)
	}
	if parsed.Destination != "example.com:443" {
		t.Errorf("Expected destination example.com:443, got %s", parsed.Destination)
	}
	if parsed.Action != "allowed" {
		t.Errorf("Expected action allowed, got %s", parsed.Action)
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		{
			name:     "valid host:port",
			input:    "example.com:443",
			wantHost: "example.com",
			wantPort: "443",
			wantErr:  false,
		},
		{
			name:     "IPv6 with port",
			input:    "[::1]:8080",
			wantHost: "::1",
			wantPort: "8080",
			wantErr:  false,
		},
		{
			name:     "no port",
			input:    "example.com",
			wantHost: "",
			wantPort: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := net.SplitHostPort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitHostPort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if host != tt.wantHost {
					t.Errorf("SplitHostPort() host = %v, want %v", host, tt.wantHost)
				}
				if port != tt.wantPort {
					t.Errorf("SplitHostPort() port = %v, want %v", port, tt.wantPort)
				}
			}
		})
	}
}

func BenchmarkIsAllowed(b *testing.B) {
	logger := NewLogger(os.Stdout)
	proxy, err := NewProxyServer("localhost:8080", logger)
	if err != nil {
		b.Fatalf("Failed to create proxy server: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proxy.isAllowed("example.com:443")
	}
}

func BenchmarkLogger(b *testing.B) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	entry := LogEntry{
		Level:       LogLevelInfo,
		Event:       "connection_attempt",
		Destination: "example.com:443",
		Action:      "allowed",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Log(entry)
	}
}
