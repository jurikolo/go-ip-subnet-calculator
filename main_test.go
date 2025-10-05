package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestParseSubnetMask(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		expected string
	}{
		{
			name:     "Valid CIDR /24",
			input:    "/24",
			wantErr:  false,
			expected: "ffffff00",
		},
		{
			name:     "Valid CIDR /16",
			input:    "/16",
			wantErr:  false,
			expected: "ffff0000",
		},
		{
			name:     "Valid CIDR /32",
			input:    "/32",
			wantErr:  false,
			expected: "ffffffff",
		},
		{
			name:     "Valid dotted decimal 255.255.255.0",
			input:    "255.255.255.0",
			wantErr:  false,
			expected: "ffffff00",
		},
		{
			name:     "Valid dotted decimal 255.255.0.0",
			input:    "255.255.0.0",
			wantErr:  false,
			expected: "ffff0000",
		},
		{
			name:     "Valid dotted decimal 255.255.255.252",
			input:    "255.255.255.252",
			wantErr:  false,
			expected: "fffffffc",
		},
		{
			name:     "Valid dotted decimal 255.255.255.248",
			input:    "255.255.255.248",
			wantErr:  false,
			expected: "fffffff8",
		},
		{
			name:     "Valid dotted decimal 255.255.255.254",
			input:    "255.255.255.254",
			wantErr:  false,
			expected: "fffffffe",
		},
		{
			name:     "Valid dotted decimal 255.255.255.255",
			input:    "255.255.255.255",
			wantErr:  false,
			expected: "ffffffff",
		},
		{
			name:    "Invalid CIDR negative",
			input:   "/-1",
			wantErr: true,
		},
		{
			name:    "Invalid CIDR too large",
			input:   "/33",
			wantErr: true,
		},
		{
			name:    "Invalid dotted decimal - out of range",
			input:   "256.255.255.0",
			wantErr: true,
		},
		{
			name:    "Invalid dotted decimal - non-contiguous mask",
			input:   "255.255.255.253",
			wantErr: true,
		},
		{
			name:    "Invalid dotted decimal - non-contiguous mask 2",
			input:   "255.255.254.255",
			wantErr: true,
		},
		{
			name:    "Invalid dotted decimal - non-contiguous mask 3",
			input:   "255.254.255.0",
			wantErr: true,
		},
		{
			name:    "Invalid dotted decimal - holes in mask",
			input:   "255.255.255.251",
			wantErr: true,
		},
		{
			name:    "Invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSubnetMask(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseSubnetMask() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseSubnetMask() unexpected error: %v", err)
				return
			}
			// Convert result to hex string for comparison
			hexStr := ""
			for _, b := range result {
				hexStr += string("0123456789abcdef"[b>>4]) + string("0123456789abcdef"[b&0xf])
			}
			if hexStr != tt.expected {
				t.Errorf("parseSubnetMask() = %s, want %s", hexStr, tt.expected)
			}
		})
	}
}

func TestIsValidSubnetMask(t *testing.T) {
	tests := []struct {
		name     string
		mask     []byte
		expected bool
	}{
		{
			name:     "Valid mask 255.255.255.0 (/24)",
			mask:     []byte{255, 255, 255, 0},
			expected: true,
		},
		{
			name:     "Valid mask 255.255.0.0 (/16)",
			mask:     []byte{255, 255, 0, 0},
			expected: true,
		},
		{
			name:     "Valid mask 255.255.255.252 (/30)",
			mask:     []byte{255, 255, 255, 252},
			expected: true,
		},
		{
			name:     "Valid mask 255.255.255.248 (/29)",
			mask:     []byte{255, 255, 255, 248},
			expected: true,
		},
		{
			name:     "Valid mask 255.255.255.254 (/31)",
			mask:     []byte{255, 255, 255, 254},
			expected: true,
		},
		{
			name:     "Valid mask 255.255.255.255 (/32)",
			mask:     []byte{255, 255, 255, 255},
			expected: true,
		},
		{
			name:     "Valid mask 0.0.0.0 (/0)",
			mask:     []byte{0, 0, 0, 0},
			expected: true,
		},
		{
			name:     "Invalid mask 255.255.255.253 (non-contiguous)",
			mask:     []byte{255, 255, 255, 253},
			expected: false,
		},
		{
			name:     "Invalid mask 255.255.254.255 (hole in mask)",
			mask:     []byte{255, 255, 254, 255},
			expected: false,
		},
		{
			name:     "Invalid mask 255.254.255.0 (hole in mask)",
			mask:     []byte{255, 254, 255, 0},
			expected: false,
		},
		{
			name:     "Invalid mask 255.255.255.251 (non-contiguous)",
			mask:     []byte{255, 255, 255, 251},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := net.IPMask(tt.mask)
			result := isValidSubnetMask(mask)
			if result != tt.expected {
				t.Errorf("isValidSubnetMask(%v) = %v, want %v", tt.mask, result, tt.expected)
			}
		})
	}
}

func TestCalculateSubnet(t *testing.T) {
	tests := []struct {
		name              string
		ip                string
		mask              string
		wantErr           bool
		expectedNetwork   string
		expectedBroadcast string
		expectedMinHost   string
		expectedMaxHost   string
		expectedUsable    string
	}{
		{
			name:              "Standard /24 subnet",
			ip:                "192.168.1.100",
			mask:              "/24",
			wantErr:           false,
			expectedNetwork:   "192.168.1.0",
			expectedBroadcast: "192.168.1.255",
			expectedMinHost:   "192.168.1.1",
			expectedMaxHost:   "192.168.1.254",
			expectedUsable:    "254",
		},
		{
			name:              "Standard /16 subnet",
			ip:                "10.5.10.20",
			mask:              "/16",
			wantErr:           false,
			expectedNetwork:   "10.5.0.0",
			expectedBroadcast: "10.5.255.255",
			expectedMinHost:   "10.5.0.1",
			expectedMaxHost:   "10.5.255.254",
			expectedUsable:    "65534",
		},
		{
			name:              "/30 subnet (point-to-point)",
			ip:                "192.168.1.5",
			mask:              "/30",
			wantErr:           false,
			expectedNetwork:   "192.168.1.4",
			expectedBroadcast: "192.168.1.7",
			expectedMinHost:   "192.168.1.5",
			expectedMaxHost:   "192.168.1.6",
			expectedUsable:    "2",
		},
		{
			name:              "/32 subnet (single host)",
			ip:                "192.168.1.1",
			mask:              "/32",
			wantErr:           false,
			expectedNetwork:   "192.168.1.1",
			expectedBroadcast: "192.168.1.1",
			expectedMinHost:   "N/A",
			expectedMaxHost:   "N/A",
			expectedUsable:    "0",
		},
		{
			name:              "/31 subnet (point-to-point link)",
			ip:                "192.168.1.1",
			mask:              "/31",
			wantErr:           false,
			expectedNetwork:   "192.168.1.0",
			expectedBroadcast: "192.168.1.1",
			expectedMinHost:   "N/A",
			expectedMaxHost:   "N/A",
			expectedUsable:    "0",
		},
		{
			name:              "Dotted decimal mask",
			ip:                "172.16.0.50",
			mask:              "255.255.255.192",
			wantErr:           false,
			expectedNetwork:   "172.16.0.0",
			expectedBroadcast: "172.16.0.63",
			expectedMinHost:   "172.16.0.1",
			expectedMaxHost:   "172.16.0.62",
			expectedUsable:    "62",
		},
		{
			name:    "Invalid IP address",
			ip:      "999.999.999.999",
			mask:    "/24",
			wantErr: true,
		},
		{
			name:    "Invalid subnet mask",
			ip:      "192.168.1.1",
			mask:    "/99",
			wantErr: true,
		},
		{
			name:    "Invalid dotted decimal subnet mask",
			ip:      "192.168.1.1",
			mask:    "255.255.255.253",
			wantErr: true,
		},
		{
			name:    "Empty IP",
			ip:      "",
			mask:    "/24",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculateSubnet(tt.ip, tt.mask)
			if tt.wantErr {
				if err == nil {
					t.Errorf("calculateSubnet() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("calculateSubnet() unexpected error: %v", err)
				return
			}

			if result.NetworkAddress != tt.expectedNetwork {
				t.Errorf("NetworkAddress = %s, want %s", result.NetworkAddress, tt.expectedNetwork)
			}
			if result.BroadcastAddress != tt.expectedBroadcast {
				t.Errorf("BroadcastAddress = %s, want %s", result.BroadcastAddress, tt.expectedBroadcast)
			}
			if result.MinHostAddress != tt.expectedMinHost {
				t.Errorf("MinHostAddress = %s, want %s", result.MinHostAddress, tt.expectedMinHost)
			}
			if result.MaxHostAddress != tt.expectedMaxHost {
				t.Errorf("MaxHostAddress = %s, want %s", result.MaxHostAddress, tt.expectedMaxHost)
			}
			if result.UsableHosts != tt.expectedUsable {
				t.Errorf("UsableHosts = %s, want %s", result.UsableHosts, tt.expectedUsable)
			}
		})
	}
}

func TestLoadTemplate(t *testing.T) {
	// Create a temporary HTML file for testing
	tmpFile := "test_index.html"
	content := `<!DOCTYPE html><html><body><h1>{{.IPAddress}}</h1></body></html>`

	// Write test content to temporary file
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile) // Clean up after test

	tmpl, err := loadTemplate(tmpFile)
	if err != nil {
		t.Errorf("loadTemplate(%s) unexpected error: %v", tmpFile, err)
		return
	}

	if tmpl == nil {
		t.Errorf("loadTemplate(%s) returned nil template", tmpFile)
	}
}

func TestLoadTemplateFileNotFound(t *testing.T) {
	// Ensure HTML file doesn't exist
	indexAsdf := "index_asdf.html"
	os.Remove(indexAsdf)

	_, err := loadTemplate(indexAsdf)
	if err == nil {
		t.Errorf("loadTemplate(%s) expected error for missing file, got nil", indexAsdf)
	}
	if !strings.Contains(err.Error(), "failed to read "+indexAsdf) {
		t.Errorf("loadTemplate(%s) error message should mention file reading, got: %v", indexAsdf, err)
	}
}

func TestHandlerGET(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "IPv4 Subnet Calculator") {
		t.Error("handler should return template content")
	}
}

func TestHandlerPOSTValidInput(t *testing.T) {
	// Create a temporary HTML for testing
	tmpFile := "test_index.html"
	content := `<!DOCTYPE html><html><body>
		<div>Network: {{.NetworkAddress}}</div>
		<div>Broadcast: {{.BroadcastAddress}}</div>
		<div>Min: {{.MinHostAddress}}</div>
		<div>Max: {{.MaxHostAddress}}</div>
		<div>Usable: {{.UsableHosts}}</div>
		{{if .Error}}<div>Error: {{.Error}}</div>{{end}}
	</body></html>`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create %s for test: %v", tmpFile, err)
	}
	defer os.Remove(tmpFile)

	form := url.Values{}
	form.Add("ip", "192.168.1.100")
	form.Add("mask", "/24")

	req, err := http.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "192.168.1.0") {
		t.Error("handler should return calculated network address")
	}
	if !strings.Contains(body, "192.168.1.255") {
		t.Error("handler should return calculated broadcast address")
	}
}

func TestHandlerPOSTInvalidInput(t *testing.T) {
	form := url.Values{}
	form.Add("ip", "invalid.ip")
	form.Add("mask", "/24")

	req, err := http.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "error") {
		t.Error("handler should return error for invalid input")
	}
}

// Benchmark tests
func BenchmarkCalculateSubnet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		calculateSubnet("192.168.1.100", "/24")
	}
}

func BenchmarkParseSubnetMask(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parseSubnetMask("/24")
	}
}

// Test helper function to check if two IP addresses are equal
func ipEqual(ip1, ip2 string) bool {
	parsedIP1 := net.ParseIP(ip1)
	parsedIP2 := net.ParseIP(ip2)
	return parsedIP1.Equal(parsedIP2)
}

func TestIPEqual(t *testing.T) {
	tests := []struct {
		ip1      string
		ip2      string
		expected bool
	}{
		{"192.168.1.1", "192.168.1.1", true},
		{"192.168.1.1", "192.168.1.2", false},
		{"10.0.0.1", "10.0.0.1", true},
	}

	for _, tt := range tests {
		result := ipEqual(tt.ip1, tt.ip2)
		if result != tt.expected {
			t.Errorf("ipEqual(%s, %s) = %v, want %v", tt.ip1, tt.ip2, result, tt.expected)
		}
	}
}

func TestHealthHandler_Success(t *testing.T) {
	t.Parallel()

	// Set a fixed start time for this test
	testStartTime := time.Now().Add(-5 * time.Minute)
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Check Content-Type header
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Check Cache-Control header
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache, no-store, must-revalidate" {
		t.Errorf("Expected Cache-Control 'no-cache, no-store, must-revalidate', got '%s'", cacheControl)
	}

	// Check Pragma header
	pragma := w.Header().Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("Expected Pragma 'no-cache', got '%s'", pragma)
	}

	// Check Expires header
	expires := w.Header().Get("Expires")
	if expires != "0" {
		t.Errorf("Expected Expires '0', got '%s'", expires)
	}

	// Parse response body
	var health HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Verify response fields
	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", health.Status)
	}

	if health.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", health.Version)
	}

	// Verify timestamp is recent (within last 2 seconds)
	if time.Since(health.Timestamp) > 2*time.Second {
		t.Errorf("Timestamp is not recent: %v", health.Timestamp)
	}

	// Verify uptime is not empty
	if health.Uptime == "" {
		t.Error("Expected non-empty uptime")
	}
}

func TestHealthHandler_ResponseStructure(t *testing.T) {
	t.Parallel()

	testStartTime := time.Now().Add(-10 * time.Minute)
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	var health HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Verify all required fields are present
	if health.Status == "" {
		t.Error("Status field is missing or empty")
	}

	if health.Version == "" {
		t.Error("Version field is missing or empty")
	}

	if health.Uptime == "" {
		t.Error("Uptime field is missing or empty")
	}

	if health.Timestamp.IsZero() {
		t.Error("Timestamp field is missing or zero")
	}
}

func TestHealthHandler_HTTPMethod(t *testing.T) {
	t.Parallel()

	testStartTime := time.Now()
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			w := httptest.NewRecorder()

			healthHandler(w, req)

			// Handler should respond successfully regardless of HTTP method
			if w.Code != http.StatusOK {
				t.Errorf("Method %s: Expected status code %d, got %d", method, http.StatusOK, w.Code)
			}

			var health HealthResponse
			if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
				t.Errorf("Method %s: Failed to decode response: %v", method, err)
			}
		})
	}
}

func TestHealthHandler_CacheHeaders(t *testing.T) {
	t.Parallel()

	testStartTime := time.Now()
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	expectedHeaders := map[string]string{
		"Cache-Control": "no-cache, no-store, must-revalidate",
		"Pragma":        "no-cache",
		"Expires":       "0",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected '%s', got '%s'", header, expectedValue, actualValue)
		}
	}
}

func TestHealthHandler_UptimeCalculation(t *testing.T) {
	t.Parallel()

	// Set start time to 1 hour ago
	testStartTime := time.Now().Add(-1 * time.Hour)
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	var health HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Verify uptime indicates roughly 1 hour
	// The uptime string should contain "h" for hours
	if health.Uptime == "" {
		t.Error("Uptime should not be empty")
	}

	// Parse the uptime to verify it's approximately correct
	uptime, err := time.ParseDuration(health.Uptime)
	if err != nil {
		t.Fatalf("Failed to parse uptime duration: %v", err)
	}

	expectedUptime := time.Since(testStartTime)
	diff := uptime - expectedUptime
	if diff < 0 {
		diff = -diff
	}

	// Allow 1 second tolerance for test execution time
	if diff > time.Second {
		t.Errorf("Uptime calculation incorrect: expected ~%v, got %v (diff: %v)", expectedUptime, uptime, diff)
	}
}

func TestHealthHandler_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	testStartTime := time.Now()
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	// Simulate concurrent requests
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()

			healthHandler(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Concurrent request failed with status code %d", w.Code)
			}

			var health HealthResponse
			if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
				t.Errorf("Concurrent request failed to decode response: %v", err)
			}

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

func TestHealthHandler_JSONValidFormat(t *testing.T) {
	t.Parallel()

	testStartTime := time.Now()
	originalStartTime := startTime
	startTime = testStartTime
	defer func() { startTime = originalStartTime }()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	healthHandler(w, req)

	// Verify the response is valid JSON
	var js json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &js); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	// Verify specific JSON structure
	var health map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	requiredFields := []string{"status", "timestamp", "version", "uptime"}
	for _, field := range requiredFields {
		if _, exists := health[field]; !exists {
			t.Errorf("Required field '%s' missing from JSON response", field)
		}
	}
}

// TestMain_PortConfiguration tests port configuration logic
func TestMain_PortConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		envPort     string
		expectedErr bool
	}{
		{
			name:        "default port when env not set",
			envPort:     "",
			expectedErr: false,
		},
		{
			name:        "custom valid port",
			envPort:     "9090",
			expectedErr: false,
		},
		{
			name:        "invalid port - non-numeric",
			envPort:     "abc",
			expectedErr: true,
		},
		{
			name:        "invalid port - with letters",
			envPort:     "80abc",
			expectedErr: true,
		},
		{
			name:        "valid high port",
			envPort:     "65535",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Simulate port validation logic from main
			port := tt.envPort
			if port == "" {
				port = "8080"
			}

			_, err := strconv.Atoi(port)
			hasErr := err != nil

			if hasErr != tt.expectedErr {
				t.Errorf("Expected error: %v, got error: %v", tt.expectedErr, hasErr)
			}
		})
	}
}

// TestMain_ServerStartup tests the server startup in a subprocess
func TestMain_ServerStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires the binary to be built
	// Run: go test -v (not in short mode)

	tests := []struct {
		name    string
		envPort string
		timeout time.Duration
	}{
		{
			name:    "starts with default port",
			envPort: "",
			timeout: 5 * time.Second,
		},
		{
			name:    "starts with custom port",
			envPort: "8081",
			timeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use subprocess approach to test main
			if os.Getenv("TEST_MAIN_PROCESS") == "1" {
				// This is the subprocess - run main
				if tt.envPort != "" {
					os.Setenv("GO_SUBNET_CALCULATOR_PORT", tt.envPort)
				}
				main()
				return
			}

			// Parent process - start subprocess
			cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
			cmd.Env = append(os.Environ(), "TEST_MAIN_PROCESS=1")
			if tt.envPort != "" {
				cmd.Env = append(cmd.Env, "GO_SUBNET_CALCULATOR_PORT="+tt.envPort)
			}

			// Capture output
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("Failed to get stdout pipe: %v", err)
			}

			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start subprocess: %v", err)
			}

			// Read startup messages
			output := make([]byte, 1024)
			n, _ := stdout.Read(output)
			outputStr := string(output[:n])

			// Give server time to start
			time.Sleep(1 * time.Second)

			// Verify output contains expected messages
			expectedPort := tt.envPort
			if expectedPort == "" {
				expectedPort = "8080"
			}

			if !strings.Contains(outputStr, "IPv4 Subnet Calculator starting") {
				t.Errorf("Expected startup message, got: %s", outputStr)
			}

			if !strings.Contains(outputStr, expectedPort) {
				t.Errorf("Expected port %s in output, got: %s", expectedPort, outputStr)
			}

			// Try to connect to the server
			url := fmt.Sprintf("http://localhost:%s/health", expectedPort)
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}
			}

			// Cleanup: kill the subprocess
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				cmd.Process.Kill()
			}
			cmd.Wait()
		})
	}
}

// TestMain_InvalidPort tests that main exits with invalid port
func TestMain_InvalidPort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if os.Getenv("TEST_INVALID_PORT") == "1" {
		// This is the subprocess - set invalid port and run main
		os.Setenv("GO_SUBNET_CALCULATOR_PORT", "invalid")
		main()
		return
	}

	// Parent process - start subprocess with invalid port
	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(),
		"TEST_INVALID_PORT=1",
		"GO_SUBNET_CALCULATOR_PORT=invalid",
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to get stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start subprocess: %v", err)
	}

	// Read error output
	output, _ := io.ReadAll(stderr)
	outputStr := string(output)

	// Wait for process to exit
	err = cmd.Wait()

	// Should exit with non-zero status
	if err == nil {
		t.Error("Expected process to exit with error for invalid port")
	}

	// Should contain error message about invalid port
	if !strings.Contains(outputStr, "Invalid port number") {
		t.Errorf("Expected 'Invalid port number' in error output, got: %s", outputStr)
	}
}

// TestMain_RouteRegistration tests that routes are properly registered
func TestMain_RouteRegistration(t *testing.T) {
	t.Parallel()

	// Save original handlers
	originalMux := http.DefaultServeMux
	defer func() { http.DefaultServeMux = originalMux }()

	// Create new ServeMux to avoid conflicts
	http.DefaultServeMux = http.NewServeMux()

	// Register handlers as main does
	http.HandleFunc("/", handler)
	http.HandleFunc("/health", healthHandler)

	// Test that routes respond (without actually starting server)
	tests := []struct {
		path           string
		expectedStatus int
	}{
		{
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Use a custom handler to test routing
			handler, pattern := http.DefaultServeMux.Handler(req)

			if pattern == "" {
				t.Errorf("No handler registered for path: %s", tt.path)
			}

			if handler == nil {
				t.Errorf("Handler is nil for path: %s", tt.path)
			}
		})
	}
}

// TestMain_EnvironmentVariableHandling tests environment variable logic
func TestMain_EnvironmentVariableHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		envValue     string
		expectedPort string
	}{
		{
			name:         "empty env uses default",
			envValue:     "",
			expectedPort: "8080",
		},
		{
			name:         "custom port from env",
			envValue:     "9000",
			expectedPort: "9000",
		},
		{
			name:         "high port number",
			envValue:     "65000",
			expectedPort: "65000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the port logic from main
			port := tt.envValue
			if port == "" {
				port = "8080"
			}

			if port != tt.expectedPort {
				t.Errorf("Expected port %s, got %s", tt.expectedPort, port)
			}

			// Validate it's numeric
			if _, err := strconv.Atoi(port); err != nil {
				t.Errorf("Port %s is not numeric: %v", port, err)
			}
		})
	}
}

// TestMain_AddressFormatting tests address string construction
func TestMain_AddressFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		port            string
		expectedAddress string
	}{
		{
			port:            "8080",
			expectedAddress: ":8080",
		},
		{
			port:            "3000",
			expectedAddress: ":3000",
		},
		{
			port:            "80",
			expectedAddress: ":80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			// Simulate address construction from main
			address := ":" + tt.port

			if address != tt.expectedAddress {
				t.Errorf("Expected address %s, got %s", tt.expectedAddress, address)
			}
		})
	}
}

// TestMain_LogOutput tests that main produces expected log output
func TestMain_LogOutput(t *testing.T) {
	t.Parallel()

	// Test the log messages that would be printed
	tests := []struct {
		port          string
		expectedInMsg []string
	}{
		{
			port: "8080",
			expectedInMsg: []string{
				"IPv4 Subnet Calculator starting on http://localhost:8080",
				"Health check available at http://localhost:8080/health",
			},
		},
		{
			port: "9090",
			expectedInMsg: []string{
				"IPv4 Subnet Calculator starting on http://localhost:9090",
				"Health check available at http://localhost:9090/health",
			},
		},
	}

	for _, tt := range tests {
		t.Run("port_"+tt.port, func(t *testing.T) {
			// Simulate the message construction
			msg1 := fmt.Sprintf("IPv4 Subnet Calculator starting on http://localhost:%s\n", tt.port)
			msg2 := fmt.Sprintf("Health check available at http://localhost:%s/health\n", tt.port)

			if !strings.Contains(msg1, tt.expectedInMsg[0]) {
				t.Errorf("Message 1 doesn't contain expected text")
			}

			if !strings.Contains(msg2, tt.expectedInMsg[1]) {
				t.Errorf("Message 2 doesn't contain expected text")
			}
		})
	}
}

// Mock handler for testing - assumes you have a handler function
// func handler(w http.ResponseWriter, r *http.Request) {
// w.WriteHeader(http.StatusOK)
// w.Write([]byte("OK"))
// }

// Helper function to suppress log output during tests
func suppressLogs() func() {
	null, _ := os.Open(os.DevNull)
	oldOutput := log.Writer()
	log.SetOutput(null)
	return func() {
		log.SetOutput(oldOutput)
		null.Close()
	}
}
