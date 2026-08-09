package work

import (
	"sync"
)

// todo, tie queues or go routines to this somehow? Should I?
type queuer struct {
	//todo, change arg order? in out seems more intuitive
	outboundWorkChannel chan WorkItem
	inboundWorkChannel  chan WorkItem
	backlogMutex        sync.Mutex
	backlog             []WorkItem
}

// todo, change arg order? in out seems more intuitive
func NewQueuer() *queuer {
	outboundWorkChannel := make(chan WorkItem)
	inboundWorkChannel := make(chan WorkItem)

	return &queuer{
		outboundWorkChannel: outboundWorkChannel,
		inboundWorkChannel:  inboundWorkChannel,
		backlogMutex:        sync.Mutex{},
		backlog:             []WorkItem{},
	}
}

// todo, need to prevent starts when already started
func (q *queuer) Start() {
	workPlacedInBacklog := make(chan struct{})

	//todo, this is broken because we can run in an order such that the second routine waits for an item to be queued as the first routine is throwing the item into the backlog, causing a deadlock as the backlog dequeuer will never get unblocked from the first item
	//todo, also, need to remove the prints down below

	// incoming data to write out or push into backlog
	go func() {
		//ensure the first piece of work is placed in the backlog so that the backlog advocate routine
		//will always pick up the first piece of work and won't stall waiting for a data ready signal
		//when this routine skipped over the signal that data is ready in placeWorkInBacklog because
		//the advocate routine got to the wait line second
		firstItem := <-q.inboundWorkChannel
		q.placeWorkInBacklog(firstItem, workPlacedInBacklog)

		//outgoing data from backlog
		go q.backlogWorkAdvocate(workPlacedInBacklog)

		//todo....better way to shut down this routine if we're done?
		for inWork := range q.inboundWorkChannel {
			select {
			case q.outboundWorkChannel <- inWork:
				// great, we passed work onto a worker
			default:
				q.placeWorkInBacklog(inWork, workPlacedInBacklog)
			}
		}
	}()
}

// note this is an unlocked read, so the value could be incorrect immediately after it is returned
func (q *queuer) GetApproximateItemsInBacklog() int {
	return len(q.backlog)
}

func (q *queuer) placeWorkInBacklog(inWork WorkItem, workPlacedInBacklog chan struct{}) {
	q.backlogMutex.Lock()
	defer q.backlogMutex.Unlock()
	q.backlog = append(q.backlog, inWork)
	//signal data is written, just move on if channel is busy (meaning the backlog advocate routine hasn't caught up)
	select {
	case workPlacedInBacklog <- struct{}{}:
	default:
	}
}

func (q *queuer) backlogWorkAdvocate(workPlacedInBacklog chan struct{}) {
	//todo....better way to shut down this routine if we're done?
	for {
		// this check might be wrong immediately after it, but if it is it'll show a smaller number than the actual number, which is fine
		isEmpty := len(q.backlog) == 0
		if !isEmpty {
			q.backlogMutex.Lock()
			lastIndex := len(q.backlog) - 1
			workOut := q.backlog[lastIndex]
			q.backlog = q.backlog[:lastIndex]
			q.backlogMutex.Unlock()
			q.outboundWorkChannel <- workOut
			continue
		} else {
			//wait until more data is written
			<-workPlacedInBacklog
		}
	}
}
