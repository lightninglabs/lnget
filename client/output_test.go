package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lightninglabs/lnget/config"
)

// TestNewOutput tests output formatter creation.
func TestNewOutput(t *testing.T) {
	tests := []struct {
		format config.OutputFormat
	}{
		{config.OutputFormatJSON},
		{config.OutputFormatHuman},
		{""},
	}

	for _, tc := range tests {
		o := NewOutput(tc.format)
		if o == nil {
			t.Fatal("NewOutput() returned nil")
		}

		if o.format != tc.format {
			t.Errorf("NewOutput(%q).format = %q, want %q",
				tc.format, o.format, tc.format)
		}
	}
}

// TestTokenInfoStruct tests the TokenInfo struct.
func TestTokenInfoStruct(t *testing.T) {
	info := TokenInfo{
		Domain:      "example.com",
		PaymentHash: "abc123",
		AmountSat:   100,
		FeeSat:      1,
		Created:     "2024-01-01 00:00:00",
		Pending:     false,
	}

	if info.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", info.Domain, "example.com")
	}

	if info.AmountSat != 100 {
		t.Errorf("AmountSat = %d, want 100", info.AmountSat)
	}
}

// TestDownloadResultStruct tests the DownloadResult struct.
func TestDownloadResultStruct(t *testing.T) {
	result := DownloadResult{
		URL:           "https://example.com/file",
		OutputPath:    "/tmp/file",
		Size:          1024,
		Duration:      "1.5s",
		L402Paid:      true,
		L402AmountSat: 50,
		L402FeeSat:    1,
	}

	if result.URL != "https://example.com/file" {
		t.Errorf("URL = %q, want %q", result.URL, "https://example.com/file")
	}

	if !result.L402Paid {
		t.Error("L402Paid should be true")
	}
}

// TestOutputSetWriter tests setting a custom writer.
func TestOutputSetWriter(t *testing.T) {
	o := NewOutput(config.OutputFormatJSON)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	// Verify writing goes to the buffer.
	data := map[string]string{"test": "value"}

	err := o.JSON(data)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Buffer is empty after writing")
	}
}

// TestOutputJSON tests JSON output formatting.
func TestOutputJSON(t *testing.T) {
	o := NewOutput(config.OutputFormatJSON)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	data := map[string]any{
		"name":  "test",
		"value": 42,
	}

	err := o.JSON(data)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	// Parse the output JSON.
	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want 'test'", result["name"])
	}

	// JSON numbers decode as float64.
	if result["value"] != float64(42) {
		t.Errorf("value = %v, want 42", result["value"])
	}
}

// TestOutputHuman tests human-readable output formatting.
func TestOutputHuman(t *testing.T) {
	o := NewOutput(config.OutputFormatHuman)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	message := "Hello, World!"

	err := o.Human(message)
	if err != nil {
		t.Fatalf("Human() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, message) {
		t.Errorf("output = %q, want to contain %q", output, message)
	}

	// Should have a newline.
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}

// TestOutputResultJSON tests Result with JSON format.
func TestOutputResultJSON(t *testing.T) {
	o := NewOutput(config.OutputFormatJSON)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	data := DownloadResult{
		URL:      "https://example.com",
		Size:     1024,
		Duration: "1s",
	}

	err := o.Result(data)
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}

	// Should be valid JSON.
	var result DownloadResult

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.URL != data.URL {
		t.Errorf("URL = %q, want %q", result.URL, data.URL)
	}
}

// TestOutputResultHuman tests Result with human format.
func TestOutputResultHuman(t *testing.T) {
	o := NewOutput(config.OutputFormatHuman)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	data := "Simple message"

	err := o.Result(data)
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, data) {
		t.Errorf("output = %q, want to contain %q", output, data)
	}
}

// TestOutputErrorJSON tests Error with JSON format.
func TestOutputErrorJSON(t *testing.T) {
	o := NewOutput(config.OutputFormatJSON)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	testErr := errors.New("something went wrong")

	err := o.Error(testErr)
	if err != nil {
		t.Fatalf("Error() error = %v", err)
	}

	// Parse the JSON output.
	var result map[string]any

	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result["error"] != true {
		t.Error("error field should be true")
	}

	if result["message"] != testErr.Error() {
		t.Errorf("message = %v, want %q", result["message"], testErr.Error())
	}
}

// TestOutputErrorHuman tests Error with human format.
func TestOutputErrorHuman(t *testing.T) {
	o := NewOutput(config.OutputFormatHuman)

	var buf bytes.Buffer
	o.SetWriter(&buf)

	testErr := errors.New("something went wrong")

	err := o.Error(testErr)
	if err != nil {
		t.Fatalf("Error() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Error:") {
		t.Errorf("output = %q, want to contain 'Error:'", output)
	}

	if !strings.Contains(output, testErr.Error()) {
		t.Errorf("output = %q, want to contain %q", output, testErr.Error())
	}
}

// TestBackendStatusStruct tests the BackendStatus struct.
func TestBackendStatusStruct(t *testing.T) {
	status := BackendStatus{
		Type:          "lnd",
		Connected:     true,
		NodePubKey:    "02abc...",
		Alias:         "test-node",
		Network:       "mainnet",
		SyncedToChain: true,
		BalanceSat:    100000,
	}

	if status.Type != "lnd" {
		t.Errorf("Type = %q, want 'lnd'", status.Type)
	}

	if !status.Connected {
		t.Error("Connected should be true")
	}

	if status.BalanceSat != 100000 {
		t.Errorf("BalanceSat = %d, want 100000", status.BalanceSat)
	}
}
