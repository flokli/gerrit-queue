package misc

import (
	"sync"

	"github.com/apex/log"
)

// RotatingLogHandler implementation.
type RotatingLogHandler struct {
	mu         sync.Mutex
	Entries    []*log.Entry
	maxEntries int
}

// NewRotatingLogHandler creates a new rotating log handler
func NewRotatingLogHandler(maxEntries int) *RotatingLogHandler {
	return &RotatingLogHandler{
		maxEntries: maxEntries,
	}
}

// HandleLog implements log.Handler.
func (h *RotatingLogHandler) HandleLog(e *log.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// drop tail if we have more entries than maxEntries
	if len(h.Entries) > h.maxEntries {
		h.Entries = append([]*log.Entry{e}, h.Entries[:(h.maxEntries-2)]...)
	} else {
		h.Entries = append([]*log.Entry{e}, h.Entries...)
	}
	return nil
}
