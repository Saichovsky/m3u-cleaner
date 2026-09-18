package iptv

import (
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	// ConcurrentChecks is the default number of parallel stream checks.
	ConcurrentChecks = 50
	// TimeoutSeconds is the default per-request stream validation timeout.
	TimeoutSeconds = 10
	// UserAgent is sent with every validation request.
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
)

// DefaultTimeout is used when a caller passes a non-positive timeout.
const DefaultTimeout = TimeoutSeconds * time.Second

// ProgressReporter logs periodic progress lines, throttled to integer
// percentage changes so long-running phases stay visible without spamming.
type ProgressReporter struct {
	lastPercent int
}

// NewProgressReporter returns a reporter whose first report is always logged,
// even at 0%.
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{lastPercent: -1}
}

func (p *ProgressReporter) Report(phase string, done, total, valid int) {
	percent := 0
	if total > 0 {
		percent = done * 100 / total
	}
	if percent <= p.lastPercent {
		return
	}
	p.lastPercent = percent
	log.Printf("%s: %d/%d (%d%%), %d valid", phase, done, total, percent, valid)
}

// ValidateChannels concurrently checks stream availability with
// ConcurrentChecks workers and returns the channels that respond HTTP 200.
func ValidateChannels(channels []Channel, timeout time.Duration, onProgress ...func(done, total, valid int)) []Channel {
	return ValidateChannelsN(channels, timeout, ConcurrentChecks, onProgress...)
}

// ValidateChannelsN is ValidateChannels with a configurable worker count.
// Non-positive timeout and worker counts fall back to the package defaults.
func ValidateChannelsN(channels []Channel, timeout time.Duration, workers int, onProgress ...func(done, total, valid int)) []Channel {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if workers <= 0 {
		workers = ConcurrentChecks
	}
	checkClient := &http.Client{Timeout: timeout}

	jobs := make(chan Channel, len(channels))
	results := make(chan *Channel, len(channels))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range jobs {
				req, err := http.NewRequest(http.MethodGet, ch.URL, nil)
				if err != nil {
					results <- nil
					continue
				}
				req.Header.Set("User-Agent", UserAgent)
				req.Header.Set("Accept", "application/vnd.apple.mpegurl, video/mp2t, */*")

				resp, err := checkClient.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					// Early cancellation: close body without reading to save bandwidth
					if cerr := resp.Body.Close(); cerr != nil {
						log.Printf("error closing response body: %v", cerr)
					}
					chCopy := ch
					results <- &chCopy
				} else {
					if resp != nil {
						if cerr := resp.Body.Close(); cerr != nil {
							log.Printf("error closing response body: %v", cerr)
						}
					}
					results <- nil
				}
			}
		}()
	}

	go func() {
		for _, ch := range channels {
			jobs <- ch
		}
		close(jobs)
	}()

	// Collect results as workers complete so progress is reported live
	// instead of only after the entire batch finishes.
	var valid []Channel
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		done := 0
		validCount := 0
		for res := range results {
			done++
			if res != nil {
				validCount++
				valid = append(valid, *res)
			}
			if len(onProgress) > 0 {
				onProgress[0](done, len(channels), validCount)
			}
		}
	}()

	wg.Wait()
	close(results)
	<-collectorDone

	return valid
}
