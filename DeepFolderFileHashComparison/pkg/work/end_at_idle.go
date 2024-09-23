package work

//todo, is the name of this file gothonic?

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"golang.org/x/sync/semaphore"
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

type endAtIdleWorkQueue struct {
	workerCount            int64
	workBacklog            []WorkItem
	workBacklogMutex       sync.Mutex // used to sync writes to the sharedWorkQueue, ensures that go routines can't fill the channel buffer and deadlock
	sharedWorkQueue        chan WorkItem
	sharedCommandQueue     chan command
	mightBeCompleteQueue   chan struct{}
	activeWorkersSemaphore semaphore.Weighted
	completeSignaler       *sync.WaitGroup
	logger                 *slog.Logger
	// a nil slice indicates that errors are not being logged, the slice being nil indicates errors are not being stored
	gathereredErrors []error
	errorWriteMutex  sync.Mutex
}

type WorkQueue interface {
	QueueWork(workItem WorkItem) error
	Start() (*readOnlyWaitGroup, error)
	GetErrors() []error
}

// workerCount+1 watcher will be the number of go routines spawned for the work, setting enableWorkBacklog to true will slow down work but will prevent deadlocks if the queue size was selected to be too small
func NewEndAtIdleWorkQueue(workerCount int64, queueSize int64, enableWorkBacklog bool, gatherErrors bool, logger *slog.Logger) (*endAtIdleWorkQueue, error) {
	if workerCount < 1 {
		return nil, errors.New("workers must be at least 1")
	} else if queueSize < 1 || queueSize < workerCount {
		return nil, errors.New("queue size must be at least 1 and at least the size of the number of worker threads")
	} else if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	var gatheredErrorsSlice []error = nil
	if gatherErrors {
		gatheredErrorsSlice = make([]error, 0)
	}

	var workBacklogSlice []WorkItem = nil
	if enableWorkBacklog {
		workBacklogSlice = make([]WorkItem, 0)
	}

	return &endAtIdleWorkQueue{
		workerCount:            workerCount,
		workBacklog:            workBacklogSlice,
		workBacklogMutex:       sync.Mutex{},
		sharedWorkQueue:        make(chan WorkItem, queueSize),
		sharedCommandQueue:     make(chan command, workerCount),
		mightBeCompleteQueue:   make(chan struct{}, workerCount),
		activeWorkersSemaphore: *semaphore.NewWeighted(workerCount),
		completeSignaler:       &sync.WaitGroup{},
		logger:                 logger,
		gathereredErrors:       gatheredErrorsSlice,
		errorWriteMutex:        sync.Mutex{},
	}, nil
}

func (eaiwq *endAtIdleWorkQueue) GetErrors() []error {
	//todo....can I copy this so I don't have to worry about accesses? get and purge maybe?
	return eaiwq.gathereredErrors
}

func (eaiwq *endAtIdleWorkQueue) RecordError(err error) {
	if eaiwq.IsRecordingErrors() {
		eaiwq.errorWriteMutex.Lock()
		defer eaiwq.errorWriteMutex.Unlock()
		eaiwq.gathereredErrors = append(eaiwq.gathereredErrors, err)
	}
}

func (eaiwq *endAtIdleWorkQueue) IsRecordingErrors() bool {
	// the fact that there's a non-nil slice here at all means we're gathering errors
	return eaiwq.gathereredErrors != nil
}

func (eaiwq *endAtIdleWorkQueue) WorkBacklogEnabled() bool {
	// the fact that there's a non-nil slice here at all means we're gathering errors
	return eaiwq.workBacklog != nil
}

func (eaiwq *endAtIdleWorkQueue) QueueWork(workItem WorkItem) {
	//todo....should I have a context timeout and shove something in the backlog if the timeout crosses to keep threads moving quickly rather than just letting one thread plug along when the backlog is enabled like I have?
	//that would allow me to make the channel unbuffered and then deadlock issues disappear entirely? (not context actually, just unbuffered....) or would doing that lead to the possibility that everyone is waiting to publish to the backlog and we still hang up?
	// maybe every worker has their own backlog, and a worker can issue a rebalance command to send work to their own backlog?
	//maybe I just want a queuer object that will write the next element to the
	eaiwq.queueWork(workItem, true)
}

// not exported function that allows for skipping of acquiring a lock if the workBacklogMutex lock is already acquired
// you MUST send a false value for acquireWorkBacklogMutex if the lock is acquired and the work backlog is enabled
func (eaiwq *endAtIdleWorkQueue) queueWork(workItem WorkItem, acquireWorkBacklogMutex bool) {
	workBacklogEnabled := eaiwq.WorkBacklogEnabled()

	if workBacklogEnabled && acquireWorkBacklogMutex {
		eaiwq.workBacklogMutex.Lock()
		defer eaiwq.workBacklogMutex.Unlock()
	}

	if workBacklogEnabled && len(eaiwq.sharedWorkQueue)+1 > cap(eaiwq.sharedWorkQueue) {
		//todo, fix this so you can turn off the warning
		eaiwq.logger.Warn("queueing work to backlog because of limited channel size, consider increasing the queue size or number of workers")
		eaiwq.workBacklog = append(eaiwq.workBacklog, workItem)
	} else {
		eaiwq.sharedWorkQueue <- workItem
	}
}

func (eaiwq *endAtIdleWorkQueue) pullItemsFromBacklogAndQueue() bool {
	if !eaiwq.WorkBacklogEnabled() {
		return false
	}
	eaiwq.workBacklogMutex.Lock()
	defer eaiwq.workBacklogMutex.Unlock()

	potentiallyRunningWorkers := eaiwq.workerCount - 1
	// see how much room we have left in the channel, take into account the potential of workers that are still running and stuffing data into the channel
	availableSpace := int64(cap(eaiwq.sharedWorkQueue)) - potentiallyRunningWorkers - int64(len(eaiwq.workBacklog))

	if availableSpace > 0 {
		startIndx := int64(len(eaiwq.workBacklog)) - availableSpace
		for indx := startIndx; indx < int64(len(eaiwq.workBacklog)); indx++ {
			eaiwq.sharedWorkQueue <- eaiwq.workBacklog[indx]
		}
		eaiwq.workBacklog = eaiwq.workBacklog[:startIndx]
		return true
	} else {
		return false
	}
}

func (eaiwq *endAtIdleWorkQueue) queueAllExits(force bool) bool {
	acquiredSemaphore := eaiwq.activeWorkersSemaphore.TryAcquire(eaiwq.workerCount)
	if acquiredSemaphore {
		defer eaiwq.activeWorkersSemaphore.Release(eaiwq.workerCount)
	}
	if (acquiredSemaphore && len(eaiwq.sharedWorkQueue) < 1) || force {
		for count := int64(0); count < eaiwq.workerCount; count++ {
			eaiwq.sharedCommandQueue <- EXIT
		}
		return true
	}
	return false
}

func (eaiwq *endAtIdleWorkQueue) processItemFromQueue(workerIndex int64, itemFromQueue WorkItem) {
	eaiwq.logger.Debug("worker doing work", "workerIndex", workerIndex)
	err := eaiwq.activeWorkersSemaphore.Acquire(context.TODO(), 1)
	if err != nil {
		eaiwq.logger.Error("error acquiring semaphore, exiting", "workerIndex", workerIndex, "error", err)
		return
	}
	defer eaiwq.activeWorkersSemaphore.Release(1)
	err = itemFromQueue()
	if err != nil {
		eaiwq.logger.Error("error while processing job", "workerIndex", workerIndex, "error", err)
		eaiwq.RecordError(err)
		if !eaiwq.IsRecordingErrors() {
			eaiwq.logger.Info("queueing exits due to job error on thread", "workerIndex", workerIndex, "numberOfExitsQueued", eaiwq.workerCount)
			eaiwq.queueAllExits(true)
			return
		}
	}

	itemsInQueue := len(eaiwq.sharedWorkQueue)
	eaiwq.logger.Debug("approx items in queue", "workerIndex", workerIndex, "itemsInQueue", itemsInQueue)
	if itemsInQueue <= 0 {
		eaiwq.logger.Debug("signaling all work might be complete", "workerIndex", workerIndex)
		eaiwq.mightBeCompleteQueue <- struct{}{}
	}

}

func (eaiwq *endAtIdleWorkQueue) Start() (*readOnlyWaitGroup, error) {
	acquiredSemaphore := eaiwq.activeWorkersSemaphore.TryAcquire(eaiwq.workerCount)
	defer eaiwq.activeWorkersSemaphore.Release(eaiwq.workerCount)
	isRunning := !acquiredSemaphore || (acquiredSemaphore && len(eaiwq.sharedWorkQueue) < 1)

	if isRunning {
		return nil, errors.New("cannot start work while work is running")
	}

	eaiwq.completeSignaler.Add(1)

	//start the watcher for when the queue empties
	go func() {
		continueLoop := true
		for continueLoop {
			<-eaiwq.mightBeCompleteQueue
			continueLoop = !eaiwq.queueAllExits(false)
		}
		eaiwq.completeSignaler.Done()
	}()

	//start the workers
	for indx := int64(0); indx < eaiwq.workerCount; indx++ {
		eaiwq.logger.Debug("launching worker", "workerIndex", indx)
		go func(workerIndex int64) {
			for {
				select {
				case command := <-eaiwq.sharedCommandQueue:
					eaiwq.logger.Debug("worker processing command", "workerIndex", workerIndex)
					switch command {
					case EXIT:
						eaiwq.logger.Debug("worker exiting", "workerIndex", workerIndex)
						return
					default:
						//only command right now is exit, so just treat this the same
						eaiwq.logger.Debug("worker exiting as a fallback default action", "workerIndex", workerIndex)
						return
					}
				case itemFromQueue := <-eaiwq.sharedWorkQueue:
					eaiwq.processItemFromQueue(workerIndex, itemFromQueue)
				}
			}
		}(indx)
	}

	roCond, err := NewReadOnlyCond(eaiwq.completeSignaler)
	if err != nil {
		return nil, err
	}

	return roCond, nil
}
