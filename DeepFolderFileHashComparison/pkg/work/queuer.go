package work

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

// todo, tie queues or go routines to this somehow? Should I?
type queuer struct {
	//todo, change arg order? in out seems more intuitive
	outboundWorkChannel chan<- WorkItem
	inboundWorkChannel  <-chan WorkItem
	backlogMutex        sync.Mutex
	backlog             []WorkItem
}

// todo, change arg order? in out seems more intuitive
func NewQueuer(outboundWorkChannel chan WorkItem, inboundWorkChannel chan WorkItem) *queuer {
	return &queuer{
		outboundWorkChannel: outboundWorkChannel,
		inboundWorkChannel:  inboundWorkChannel,
		backlogMutex:        sync.Mutex{},
		backlog:             []WorkItem{},
	}
}

func (q *queuer) Start() {
	dataWrittenSemaphore := semaphore.NewWeighted(1)

	// incoming data to write out or push into backlog
	go func() {
		//todo....better way to shut down this routine if we're done?
		for inWork := range q.inboundWorkChannel {
			select {
			case q.outboundWorkChannel <- inWork:
				// great, we passed work onto a worker
			default:
				q.backlogMutex.Lock()
				q.backlog = append(q.backlog, inWork)

				//semaphore available means data was written
				acquiredSemaphore := dataWrittenSemaphore.TryAcquire(1)
				if !acquiredSemaphore {
					dataWrittenSemaphore.Release(1)
				}
				q.backlogMutex.Unlock()
			}
		}
	}()

	//outgoing data from backlog
	go func() {
		//todo....better way to shut down this routine if we're done?
		for {
			isEmpty := len(q.backlog) == 0
			if !isEmpty {
				q.backlogMutex.Lock()
				lastIndex := len(q.backlog) - 1
				workOut := q.backlog[lastIndex]
				q.backlog = q.backlog[:lastIndex]
				if len(q.backlog) == 0 {
					dataWrittenSemaphore.Release(1)
				}
				q.backlogMutex.Unlock()
				q.outboundWorkChannel <- workOut
				continue
			} else {
				//wait until more data is written
				dataWrittenSemaphore.Acquire(context.TODO(), 1)
			}
		}
	}()
}

// note this is an unlocked read, so the value could be incorrect immediately after it is returned
func (q *queuer) GetApproximateItemsInBacklog() int {
	return len(q.backlog)
}
