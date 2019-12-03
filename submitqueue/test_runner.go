package submitqueue

// import (
// 	"testing"

// 	"github.com/apex/log"
// 	"github.com/apex/log/handlers/memory"
// 	"github.com/tweag/gerrit-queue/gerrit"
// )

// type MockClient struct {
// 	gerrit.Client
// }

// func (c *MockClient) Refresh() error {
// 	return nil
// }

// func TestNewRunner(t *testing.T) {
// 	logger := &log.Logger{
// 		Handler: memory.New(),
// 	}

// 	//TODO
// 	gerrit := &MockClient {
// 		client: nil,
// 	}
// 	submitQueueTag := "submitme"
// 	runner := NewRunner(logger, gerrit, submitQueueTag)
// }
