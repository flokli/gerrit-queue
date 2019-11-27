package submitqueue

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Problem: no inspection during the run
// Problem: record the state

// Result contains all data necessary to inspect a previous run
// This includes the Series from that run, and all Log Entries collected.
// It also implements the interface required for logrus.Hook.
type Result struct {
	LogEntries []*logrus.Entry
	Series     []Serie
	Error      error
	startTime  time.Time
	HEAD       string
}

// MakeResult produces a new Result struct,
// and initializes startTime with the current time.
func MakeResult() *Result {
	return &Result{
		startTime: time.Now(),
	}
}

// StartTime returns the startTime
func (r Result) StartTime() time.Time {
	return r.startTime
}

// EndTime returns the time of the latest log entry
func (r Result) EndTime() time.Time {
	if len(r.LogEntries) == 0 {
		return r.startTime
	}
	return r.LogEntries[len(r.LogEntries)-1].Time
}

// Fire is called by logrus on each log event,
// we collect all log entries in the struct variable
func (r *Result) Fire(entry *logrus.Entry) error {
	r.LogEntries = append(r.LogEntries, entry)
	return nil
}

// Levels is called by logrus to determine whether to Fire the handler.
// As we want to collect all log entries, we return logrus.AllLevels
func (r *Result) Levels() []logrus.Level {
	return logrus.AllLevels
}
