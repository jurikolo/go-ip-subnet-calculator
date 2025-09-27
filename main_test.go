package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
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

	// Temporarily change the filename for testing
	originalName := "index.html"
	// We'll test by creating index.html temporarily
	err = os.WriteFile(originalName, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html for test: %v", err)
	}
	defer func() {
		// Clean up - remove the test file if it was created for testing
		if _, err := os.Stat(originalName); err == nil {
			os.Remove(originalName)
		}
	}()

	tmpl, err := loadTemplate()
	if err != nil {
		t.Errorf("loadTemplate() unexpected error: %v", err)
		return
	}

	if tmpl == nil {
		t.Error("loadTemplate() returned nil template")
	}
}

func TestLoadTemplateFileNotFound(t *testing.T) {
	// Ensure index.html doesn't exist
	os.Remove("index.html")

	_, err := loadTemplate()
	if err == nil {
		t.Error("loadTemplate() expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read index.html") {
		t.Errorf("loadTemplate() error message should mention file reading, got: %v", err)
	}
}

func TestHandlerGET(t *testing.T) {
	// Create a temporary index.html for testing
	content := `<!DOCTYPE html><html><body><h1>Test</h1><form method="POST"></form></body></html>`
	err := os.WriteFile("index.html", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html for test: %v", err)
	}
	defer os.Remove("index.html")

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

	if !strings.Contains(rr.Body.String(), "Test") {
		t.Error("handler should return template content")
	}
}

func TestHandlerPOSTValidInput(t *testing.T) {
	// Create a temporary index.html for testing
	content := `<!DOCTYPE html><html><body>
		<div>Network: {{.NetworkAddress}}</div>
		<div>Broadcast: {{.BroadcastAddress}}</div>
		<div>Min: {{.MinHostAddress}}</div>
		<div>Max: {{.MaxHostAddress}}</div>
		<div>Usable: {{.UsableHosts}}</div>
		{{if .Error}}<div>Error: {{.Error}}</div>{{end}}
	</body></html>`
	err := os.WriteFile("index.html", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html for test: %v", err)
	}
	defer os.Remove("index.html")

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
	// Create a temporary index.html for testing
	content := `<!DOCTYPE html><html><body>
		{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
	</body></html>`
	err := os.WriteFile("index.html", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html for test: %v", err)
	}
	defer os.Remove("index.html")

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
