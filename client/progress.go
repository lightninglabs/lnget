package client

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Progress tracks download progress and displays a progress bar.
type Progress struct {
	// total is the total number of bytes to download.
	total int64

	// current is the current number of bytes downloaded.
	current int64

	// startTime is when the download started.
	startTime time.Time

	// lastUpdate is the last time the progress bar was updated.
	lastUpdate time.Time

	// output is where to write the progress bar.
	output io.Writer

	// quiet suppresses progress output.
	quiet bool

	mu sync.Mutex
}

// NewProgress creates a new progress tracker.
func NewProgress(quiet bool) *Progress {
	return &Progress{
		output:    os.Stderr,
		startTime: time.Now(),
		quiet:     quiet,
	}
}

// SetTotal sets the total number of bytes to download.
func (p *Progress) SetTotal(total int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.total = total
}

// Write implements io.Writer for tracking progress.
func (p *Progress) Write(b []byte) (int, error) {
	n := len(b)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.current += int64(n)
	p.render()

	return n, nil
}

// render draws the progress bar.
func (p *Progress) render() {
	if p.quiet {
		return
	}

	// Limit updates to once per 100ms.
	now := time.Now()
	if now.Sub(p.lastUpdate) < 100*time.Millisecond {
		return
	}
	p.lastUpdate = now

	// Calculate progress percentage.
	var percent float64
	if p.total > 0 {
		percent = float64(p.current) / float64(p.total) * 100
	}

	// Calculate speed.
	elapsed := time.Since(p.startTime)
	var speed float64
	if elapsed > 0 {
		speed = float64(p.current) / elapsed.Seconds()
	}

	// Format sizes.
	currentStr := formatBytes(p.current)
	totalStr := formatBytes(p.total)
	speedStr := formatBytes(int64(speed)) + "/s"

	// Calculate ETA.
	var eta string
	if p.total > 0 && speed > 0 {
		remaining := float64(p.total-p.current) / speed
		eta = formatDuration(time.Duration(remaining) * time.Second)
	} else {
		eta = "--:--"
	}

	// Build progress bar.
	barWidth := 30
	filled := int(percent / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := ""
	for i := 0; i < barWidth; i++ {
		switch {
		case i < filled:
			bar += "="
		case i == filled:
			bar += ">"
		default:
			bar += " "
		}
	}

	// Print the progress line.
	_, _ = fmt.Fprintf(p.output, "\r%s/%s [%s] %5.1f%% %s eta %s",
		currentStr, totalStr, bar, percent, speedStr, eta)
}

// Finish completes the progress bar.
func (p *Progress) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.quiet {
		// Print final progress with newline.
		elapsed := time.Since(p.startTime)
		avgSpeed := formatBytes(int64(float64(p.current)/
			elapsed.Seconds())) + "/s"
		_, _ = fmt.Fprintf(
			p.output, "\r%s downloaded in %s (%s avg)        \n",
			formatBytes(p.current),
			formatDuration(elapsed),
			avgSpeed,
		)
	}
}

// formatBytes formats bytes as human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024

	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f%ciB", float64(bytes)/float64(div),
		"KMGTPE"[exp])
}

// formatDuration formats a duration as mm:ss.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second

	return fmt.Sprintf("%02d:%02d", m, s)
}
