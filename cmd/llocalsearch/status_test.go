package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nilsherzig/llocalsearch/scraper"
)

func TestSessionStatusReporterLogsColoredSingleLineStatusOnTick(t *testing.T) {
	var output bytes.Buffer
	baseTime := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	ticker := newFakeStatusTicker()

	reporter := &sessionStatusReporter{
		writer:    &output,
		startedAt: baseTime,
		now:       func() time.Time { return baseTime.Add(1 * time.Minute) },
		ticker:    ticker,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	reporter.start()

	reporter.OnPageSaved(scraper.SessionPageEvent{
		EmbeddingDuration: 150 * time.Millisecond,
	})

	ticker.ch <- baseTime.Add(30 * time.Second)
	waitForStatusOutput(t, &output, "pages_per_minute=1.00")
	reporter.Finish()

	lines := nonEmptyLines(output.String())
	if len(lines) != 2 {
		t.Fatalf("expected one periodic line and one final line, got %d in %q", len(lines), output.String())
	}
	if !strings.Contains(lines[0], "\x1b[1;36mstatus\x1b[0m") {
		t.Fatalf("expected colored status prefix, got %q", lines[0])
	}
	if strings.Contains(lines[0], "\r") {
		t.Fatalf("expected plain newline-delimited log line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "session_elapsed=1m0s") {
		t.Fatalf("expected elapsed session time in periodic line, got %q", lines[0])
	}
}

func TestSessionStatusReporterFinishStopsTickerAndLogsFinalStatus(t *testing.T) {
	var output bytes.Buffer
	baseTime := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	ticker := newFakeStatusTicker()

	reporter := &sessionStatusReporter{
		writer:    &output,
		startedAt: baseTime,
		now:       func() time.Time { return baseTime.Add(2 * time.Minute) },
		ticker:    ticker,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	reporter.start()

	reporter.OnRequestFailed(scraper.SessionFailureEvent{
		FailedAt: baseTime.Add(90 * time.Second),
	})
	reporter.Finish()

	if !ticker.stopped {
		t.Fatal("expected ticker to stop on Finish")
	}

	lines := nonEmptyLines(output.String())
	if len(lines) != 1 {
		t.Fatalf("expected only final status line, got %d in %q", len(lines), output.String())
	}
	if !strings.Contains(lines[0], "failed_requests_last_minute=1") {
		t.Fatalf("expected failure metrics in final line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "session_elapsed=2m0s") {
		t.Fatalf("expected elapsed session time in final line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "final=true") {
		t.Fatalf("expected final marker in final line, got %q", lines[0])
	}
	if strings.Contains(output.String(), "\x1b[?25") || strings.Contains(output.String(), "\x1b[r") {
		t.Fatalf("expected no terminal control sequences, got %q", output.String())
	}
}

type fakeStatusTicker struct {
	ch      chan time.Time
	stopped bool
}

func newFakeStatusTicker() *fakeStatusTicker {
	return &fakeStatusTicker{
		ch: make(chan time.Time, 4),
	}
}

func (t *fakeStatusTicker) Chan() <-chan time.Time {
	return t.ch
}

func (t *fakeStatusTicker) Stop() {
	t.stopped = true
}

func waitForStatusOutput(t *testing.T, output *bytes.Buffer, pattern string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), pattern) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected output to contain %q, got %q", pattern, output.String())
}

func nonEmptyLines(output string) []string {
	rawLines := strings.Split(output, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
