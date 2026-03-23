package main

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nilsherzig/llocalsearch/scraper"
)

const statusInterval = 30 * time.Second

type statusTicker interface {
	Chan() <-chan time.Time
	Stop()
}

type timeTicker struct {
	*time.Ticker
}

func (t *timeTicker) Chan() <-chan time.Time {
	return t.C
}

type sessionStatusReporter struct {
	writer       io.Writer
	startedAt    time.Time
	now          func() time.Time
	ticker       statusTicker
	stop         chan struct{}
	done         chan struct{}
	stopOnce     sync.Once
	pagesScraped int
	failures     []time.Time
	mu           sync.Mutex
}

func newSessionStatusReporter(writer io.Writer) *sessionStatusReporter {
	reporter := &sessionStatusReporter{
		writer:    writer,
		startedAt: time.Now(),
		now:       time.Now,
		ticker:    &timeTicker{Ticker: time.NewTicker(statusInterval)},
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	reporter.start()
	return reporter
}

func (r *sessionStatusReporter) start() {
	go func() {
		defer close(r.done)
		if r.ticker == nil {
			return
		}

		for {
			select {
			case <-r.stop:
				return
			case <-r.ticker.Chan():
				r.logStatus(false)
			}
		}
	}()
}

func (r *sessionStatusReporter) OnPageSaved(event scraper.SessionPageEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pagesScraped++
}

func (r *sessionStatusReporter) OnRequestFailed(event scraper.SessionFailureEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failures = append(r.failures, event.FailedAt)
}

func (r *sessionStatusReporter) Finish() {
	r.stopOnce.Do(func() {
		close(r.stop)
		if r.ticker != nil {
			r.ticker.Stop()
		}
	})

	if r.done != nil {
		<-r.done
	}

	r.logStatus(true)
}

func (r *sessionStatusReporter) logStatus(final bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.writer == nil {
		return
	}

	now := time.Now()
	if r.now != nil {
		now = r.now()
	}

	_, _ = fmt.Fprintf(r.writer, "%s %s\n", coloredStatusPrefix(), r.statusLineLocked(now, final))
}

func (r *sessionStatusReporter) statusLineLocked(now time.Time, final bool) string {
	elapsed := now.Sub(r.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	elapsedMinutes := now.Sub(r.startedAt).Minutes()
	if elapsedMinutes <= 0 {
		elapsedMinutes = 1.0 / 60.0
	}

	failedLastMinute := 0
	cutoff := now.Add(-1 * time.Minute)
	for _, failedAt := range r.failures {
		if !failedAt.Before(cutoff) {
			failedLastMinute++
		}
	}

	return fmt.Sprintf(
		"session_elapsed=%s pages_per_minute=%.2f pages_scraped_in_session=%d failed_requests_per_minute=%.2f failed_requests_last_minute=%d final=%t",
		formatElapsedDuration(elapsed),
		float64(r.pagesScraped)/elapsedMinutes,
		r.pagesScraped,
		float64(len(r.failures))/elapsedMinutes,
		failedLastMinute,
		final,
	)
}

func coloredStatusPrefix() string {
	return "\x1b[1;36mstatus\x1b[0m"
}

func formatElapsedDuration(elapsed time.Duration) string {
	if elapsed < time.Millisecond {
		return "0s"
	}
	if elapsed < time.Second {
		return elapsed.Round(time.Millisecond).String()
	}
	return elapsed.Round(time.Second).String()
}
