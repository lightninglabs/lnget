package client

import (
	"bytes"
	"testing"
	"time"
)

// TestNewProgress tests progress tracker creation.
func TestNewProgress(t *testing.T) {
	p := NewProgress(false)
	if p == nil {
		t.Fatal("NewProgress() returned nil")
	}

	if p.quiet {
		t.Error("NewProgress(false) should not be quiet")
	}

	pQuiet := NewProgress(true)
	if !pQuiet.quiet {
		t.Error("NewProgress(true) should be quiet")
	}
}

// TestProgressSetTotal tests setting total bytes.
func TestProgressSetTotal(t *testing.T) {
	p := NewProgress(false)
	p.SetTotal(1000)

	if p.total != 1000 {
		t.Errorf("SetTotal() = %d, want 1000", p.total)
	}
}

// TestProgressWrite tests the Write method.
func TestProgressWrite(t *testing.T) {
	p := NewProgress(true) // Quiet mode for testing
	p.SetTotal(100)

	n, err := p.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n != 5 {
		t.Errorf("Write() = %d, want 5", n)
	}

	if p.current != 5 {
		t.Errorf("current = %d, want 5", p.current)
	}
}

// TestProgressFinish tests the Finish method.
func TestProgressFinish(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:    buf,
		startTime: time.Now(),
		quiet:     false,
		current:   1000,
		total:     1000,
	}

	p.Finish()

	// Should have output something.
	if buf.Len() == 0 {
		t.Error("Finish() did not output anything")
	}
}

// TestProgressFinishQuiet tests Finish in quiet mode.
func TestProgressFinishQuiet(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:    buf,
		startTime: time.Now(),
		quiet:     true,
		current:   1000,
		total:     1000,
	}

	p.Finish()

	// Should NOT have output anything in quiet mode.
	if buf.Len() != 0 {
		t.Error("Finish() output in quiet mode")
	}
}

// TestFormatBytes tests byte formatting.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0B"},
		{100, "100B"},
		{1023, "1023B"},
		{1024, "1.0KiB"},
		{1536, "1.5KiB"},
		{1048576, "1.0MiB"},
		{1073741824, "1.0GiB"},
	}

	for _, tc := range tests {
		result := formatBytes(tc.bytes)
		if result != tc.expected {
			t.Errorf("formatBytes(%d) = %q, want %q",
				tc.bytes, result, tc.expected)
		}
	}
}

// TestFormatDuration tests duration formatting.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00"},
		{30 * time.Second, "00:30"},
		{60 * time.Second, "01:00"},
		{90 * time.Second, "01:30"},
		{5 * time.Minute, "05:00"},
		{65 * time.Second, "01:05"},
	}

	for _, tc := range tests {
		result := formatDuration(tc.duration)
		if result != tc.expected {
			t.Errorf("formatDuration(%v) = %q, want %q",
				tc.duration, result, tc.expected)
		}
	}
}

// TestProgressRender tests the progress bar rendering.
func TestProgressRender(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:     buf,
		startTime:  time.Now().Add(-time.Second), // Started 1 second ago.
		lastUpdate: time.Time{},                  // Force update (no rate limit).
		quiet:      false,
		current:    512,
		total:      1024,
	}

	// Call render directly.
	p.render()

	output := buf.String()
	if len(output) == 0 {
		t.Error("render() did not output anything")
	}

	// Should contain progress indicators.
	if !bytes.Contains(buf.Bytes(), []byte("%")) {
		t.Error("output should contain percentage indicator")
	}
}

// TestProgressRenderQuiet tests that render does nothing in quiet mode.
func TestProgressRenderQuiet(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:     buf,
		startTime:  time.Now(),
		lastUpdate: time.Time{},
		quiet:      true, // Quiet mode.
		current:    512,
		total:      1024,
	}

	p.render()

	if buf.Len() != 0 {
		t.Error("render() should not output in quiet mode")
	}
}

// TestProgressRenderRateLimit tests the rate limiting of render.
func TestProgressRenderRateLimit(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:     buf,
		startTime:  time.Now().Add(-time.Second),
		lastUpdate: time.Now(), // Just updated.
		quiet:      false,
		current:    100,
		total:      1000,
	}

	p.render()

	// Should not output due to rate limiting.
	if buf.Len() != 0 {
		t.Error("render() should be rate limited")
	}
}

// TestProgressRenderUnknownTotal tests render with unknown total size.
func TestProgressRenderUnknownTotal(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:     buf,
		startTime:  time.Now().Add(-time.Second),
		lastUpdate: time.Time{},
		quiet:      false,
		current:    1024,
		total:      0, // Unknown total.
	}

	p.render()

	output := buf.String()
	if len(output) == 0 {
		t.Error("render() did not output anything")
	}

	// Should show ETA as --:--
	if !bytes.Contains(buf.Bytes(), []byte("--:--")) {
		t.Error("output should contain '--:--' for unknown ETA")
	}
}

// TestProgressWriteNonQuiet tests Write with progress bar output.
func TestProgressWriteNonQuiet(t *testing.T) {
	buf := &bytes.Buffer{}
	p := &Progress{
		output:     buf,
		startTime:  time.Now().Add(-time.Second),
		lastUpdate: time.Time{},
		quiet:      false,
		current:    0,
		total:      100,
	}

	n, err := p.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n != 5 {
		t.Errorf("Write() = %d, want 5", n)
	}

	if p.current != 5 {
		t.Errorf("current = %d, want 5", p.current)
	}

	// Should have output progress.
	if buf.Len() == 0 {
		t.Error("Write() should have rendered progress")
	}
}
