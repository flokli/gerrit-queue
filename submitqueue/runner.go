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

func NewRunner(sq *SubmitQueue) *Runner {
	return &Runner{
		submitQueue: sq,
		results:     []*Result{},
	}
}

// For the frontend to consume the data
// TODO: extend to return all the submitQueue results
func (r *Runner) GetResults() (*time.Time, []*Result) {
	r.mut.Lock()
	defer r.mut.Unlock()
	return r.currentlyRunning, r.results
}

// GetSubmitQueue returns the submit queue object, to be consumed by the frontend
func (r *Runner) GetSubmitQueue() (*SubmitQueue) {
	return r.submitQueue
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
