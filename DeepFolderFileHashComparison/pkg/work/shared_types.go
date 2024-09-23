package work

import (
	"errors"
	"sync"
)

type command int
type WorkItem func() error

const (
	EXIT command = iota
)

type readOnlyWaitGroup struct {
	waitGroup *sync.WaitGroup
}

func NewReadOnlyCond(waitGroup *sync.WaitGroup) (*readOnlyWaitGroup, error) {
	if waitGroup == nil {
		return nil, errors.New("waitGroup must not be nil")
	}

	return &readOnlyWaitGroup{waitGroup: waitGroup}, nil
}

func (roc *readOnlyWaitGroup) Wait() {
	roc.waitGroup.Wait()
}

type WorkQueue interface {
	QueueWork(workItem WorkItem)
	Start() (*readOnlyWaitGroup, error)
	GetErrors() []error
}
