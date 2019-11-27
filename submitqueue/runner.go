package submitqueue

import (
	"sync"
	"time"
)

// Runner supervises the submit queue and records historical data about it
type Runner struct {
	mut              sync.Mutex
	submitQueue      *SubmitQueue
	currentlyRunning *time.Time
	results          []*Result
}

// NewRunner initializes a new runner object
func NewRunner(sq *SubmitQueue) *Runner {
	return &Runner{
		submitQueue: sq,
		results:     []*Result{},
	}
}

// GetState returns a copy of all the state for the frontend
func (r *Runner) GetState() (SubmitQueue, *time.Time, []*Result) {
	r.mut.Lock()
	defer r.mut.Unlock()
	return *r.submitQueue, r.currentlyRunning, r.results
}

// Trigger starts a new batch job
// TODO: make sure only one batch job is started at the same time
// if a batch job is already started, ignore the newest request
// TODO: be more granular in dry-run mode
func (r *Runner) Trigger(fetchOnly bool) {
	r.mut.Lock()
	if r.currentlyRunning != nil {
		return
	}
	now := time.Now()
	r.currentlyRunning = &now
	r.mut.Unlock()

	defer func() {
		r.mut.Lock()
		r.currentlyRunning = nil
		r.mut.Unlock()
	}()

	result := r.submitQueue.Run(fetchOnly)

	r.mut.Lock()
	// drop tail if size > 10
	if len(r.results) > 10 {
		r.results = append([]*Result{result}, r.results[:9]...)
	} else {
		r.results = append([]*Result{result}, r.results...)
	}
	r.mut.Unlock()
}
