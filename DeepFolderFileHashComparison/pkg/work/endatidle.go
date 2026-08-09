package work

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"golang.org/x/sync/semaphore"
)

type endAtIdleWorkQueue struct {
	workerCount            int64
	queuer                 *queuer
	isRunningMutex         sync.Mutex
	sharedCommandQueue     chan command
	mightBeCompleteQueue   chan struct{}
	activeWorkersSemaphore semaphore.Weighted
	completeSignaler       *sync.WaitGroup
	logger                 *slog.Logger
	// a nil slice indicates that errors are not being logged, the slice being nil indicates errors are not being stored
	gathereredErrors []error
	errorWriteMutex  sync.Mutex
}

// workerCount+1 watcher+1 queue thread + 1 backlog thread (handling overflows so there are no deadlocks) will be the number of go routines spawned for the work
func NewEndAtIdleWorkQueue(workerCount int64, gatherErrors bool, logger *slog.Logger) (*endAtIdleWorkQueue, error) {
	if workerCount < 1 {
		return nil, errors.New("workers must be at least 1")
	} else if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	var gatheredErrorsSlice []error = nil
	if gatherErrors {
		gatheredErrorsSlice = make([]error, 0)
	}

	queuer := NewQueuer()
	queuer.Start()

	return &endAtIdleWorkQueue{
		workerCount:            workerCount,
		queuer:                 queuer,
		isRunningMutex:         sync.Mutex{},
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

func (eaiwq *endAtIdleWorkQueue) QueueWork(workItem WorkItem) {
	eaiwq.queuer.inboundWorkChannel <- workItem
}

func (eaiwq *endAtIdleWorkQueue) queueAllExits(force bool) bool {
	acquiredSemaphore := eaiwq.activeWorkersSemaphore.TryAcquire(eaiwq.workerCount)
	if acquiredSemaphore {
		defer eaiwq.activeWorkersSemaphore.Release(eaiwq.workerCount)
	}
	if (acquiredSemaphore && eaiwq.queuer.GetApproximateItemsInBacklog() < 1) || force {
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

	itemsInQueue := eaiwq.queuer.GetApproximateItemsInBacklog()
	eaiwq.logger.Debug("approx items in queue", "workerIndex", workerIndex, "itemsInQueue", itemsInQueue)
	if itemsInQueue <= 0 {
		eaiwq.logger.Debug("signaling all work might be complete", "workerIndex", workerIndex)
		eaiwq.mightBeCompleteQueue <- struct{}{}
	}

}

func (eaiwq *endAtIdleWorkQueue) Start() (*readOnlyWaitGroup, error) {
	lockAcquired := eaiwq.isRunningMutex.TryLock()
	if !lockAcquired {
		return nil, errors.New("cannot start work while work is running")
	} else {
		defer eaiwq.isRunningMutex.Unlock()
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
				case itemFromQueue := <-eaiwq.queuer.outboundWorkChannel:
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
