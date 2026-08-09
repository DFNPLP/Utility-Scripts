package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	errorhandling "parks-young.com/dffhc/pkg/error_handling"
	"parks-young.com/dffhc/pkg/hashing"
	"parks-young.com/dffhc/pkg/persistence"
	"parks-young.com/dffhc/pkg/work"
)

/* steps
start using real DB
develop ui
     allow files to be moved...need to work out how to select locations
	 allow files to be deleted
linux support
deal with all todos


future:
save results
resume partial
update from partial?
*/

//https://github.com/fyne-io/fyne/blob/master/README.md for UI, maybe? Or try to build it into electron/react native somehow?

//todo, split worker off to be independent and not care about work type
//todo, move worker code into a pkg file, file operations into a pkg file, storage operations into a package file, and comparison operations into a package file
//todo, rewrite code that does semaphores to be functions so I can defer in them
//todo, deal with error outputs
//todo, use a logger that will log out text and allow user to specify log level

//todo, I think I want to publish this? https://go.dev/doc/modules/publishing

// todo, make this simple, do the hash, make the comparison a different function
func main() {
	// var quiet bool
	//var continueIfStalled bool
	// var followSymlinks bool //todo not sure if this is relevant anymore

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	errorHandler, err := errorhandling.NewLogOrPanicErrorHandler(false, logger, true)
	if err != nil {
		logger.Error("issue creating error handler", "error", err)
		return
	}

	//declare args
	var fromPathString string
	var toPathString string
	var workerThreads int64
	var gatherErrors bool

	// read in args
	flag.StringVar(&fromPathString, "frompath", "", "path to start the hashing for the \"source\" directory")
	flag.StringVar(&toPathString, "topath", "", "path to start the hashing for the \"target\" directory")
	flag.Int64Var(&workerThreads, "workers", 2, "number of worker threads, minimum 1")
	flag.BoolVar(&gatherErrors, "gathererrors", false, "false for the application should exit immediately after an error is encountered (such as when trying to read a file), true for continuing and logging information about the error")
	flag.Parse()

	// process args
	cleanedFromPath, err := filepath.Abs(fromPathString)
	if err != nil {
		logger.Error("provided frompath must be translatable into an absolute path", "path", fromPathString, "error", err)
		return
	}
	rootFromFolderInfo, err := os.Stat(cleanedFromPath)
	if err != nil {
		logger.Error("unable to validate frompath", "path", cleanedFromPath, "error", err)
		return
	} else if !rootFromFolderInfo.IsDir() {
		logger.Error("frompath must be a valid directory", "path", cleanedFromPath, "error", err)
		return
	}

	cleanedToPath, err := filepath.Abs(toPathString)
	if err != nil {
		logger.Error("provided topath must be translatable into an absolute path", "path", toPathString, "error", err)
		return
	}
	rootToFolderInfo, err := os.Stat(cleanedToPath)
	if err != nil {
		logger.Error("unable to validate topath", "path", cleanedToPath, "error", err)
		return
	} else if !rootToFolderInfo.IsDir() {
		logger.Error("topath must be a valid directory", "path", cleanedToPath, "error", err)
		return
	}

	workerQueue, err := work.NewEndAtIdleWorkQueue(workerThreads, gatherErrors, logger)
	if err != nil {
		logger.Error("issue creating worker queue", "error", err)
		return
	}

	// value of 0 means queued, value of >= 1 means it's been processed at least once
	queuedOrVisitedFromFolderSet := map[string]struct{}{}
	visitedCheckFromFolder := sync.Mutex{}
	queuedOrVisitedToFolderSet := map[string]struct{}{}
	visitedCheckToFolder := sync.Mutex{}

	//add item to work queue and mark it as visited
	queuedOrVisitedFromFolderSet[cleanedFromPath] = struct{}{}

	hasher := hashing.NewSha256Hasher()

	dataAccessLayer, err := persistence.NewFimulcrumDbAccessLayerWithDefaults("test.db", hasher.HashSizeInBytes())
	if err != nil {
		//todo....this might be a valid panic?
		panic(err)
	}
	defer dataAccessLayer.CloseDb()

	workerQueue.QueueWork(
		getNextWorkItem(
			rootFromFolderInfo,
			cleanedFromPath,
			workerQueue,
			&visitedCheckFromFolder,
			hasher,
			errorHandler,
			dataAccessLayer.PersistNewSourceHashRecord,
			queuedOrVisitedFromFolderSet,
			logger,
		),
	)
	logger.Debug("copy from hashing work queued")

	workerQueue.QueueWork(
		//todo, rework the order of these args?
		getNextWorkItem(
			rootToFolderInfo,
			cleanedToPath,
			workerQueue,
			&visitedCheckToFolder,
			hasher,
			errorHandler,
			dataAccessLayer.PersistNewTargetHashRecord,
			queuedOrVisitedToFolderSet,
			logger,
		),
	)
	logger.Debug("copy to hashing work queued")

	completeCond, err := workerQueue.Start()
	if err != nil {
		logger.Error("issue starting processing of work items", "error", err)
		return
	} else {
		logger.Debug("worker queue started")
	}

	logger.Debug("waiting on work queue completion")
	completeCond.Wait()
	um1, err := dataAccessLayer.GetSourceHashRecords(100)
	if err != nil {
		panic(err)
	}
	print(len(um1))
	um2, err := dataAccessLayer.GetTargetHashRecords(100)
	if err != nil {
		panic(err)
	}
	print(len(um2))

	print("hi!")
}

func getNextWorkItem(
	fileInfo fs.FileInfo,
	pathForHashOrWalk string,
	workQueue work.WorkQueue,
	visitedElementsLock *sync.Mutex,
	hasher hashing.Hasher,
	errorHandler errorhandling.ErrorHandler,
	storeDataFunction func(*persistence.HashData) error,
	queuedOrVisitedSet map[string]struct{},
	logger *slog.Logger,
) work.WorkItem {

	return func() error {
		isSymlink := fileInfo.Mode()&os.ModeSymlink > 0

		if isSymlink {
			//todo need to fix this
			panic("sym links not supported currently")
		} else if fileInfo.IsDir() {
			err := filepath.WalkDir(pathForHashOrWalk, func(path string, d fs.DirEntry, err error) error {
				//todo, handle err being passed in as non-nil: https://pkg.go.dev/io/fs#WalkDirFunc

				var returnValue error = nil

				fullPath, err := filepath.Abs(path)
				if err != nil {
					//todo need to handle this err
					panic(err)
				}
				folderInfo, err := os.Stat(fullPath)
				if err != nil {
					//todo need to handle this err, just skipping for now....I've seen a file not found error on certain links from here
					//panic(err)
					return returnValue
				}

				visitedElementsLock.Lock()
				defer visitedElementsLock.Unlock()
				if _, visited := queuedOrVisitedSet[fullPath]; !visited {
					queuedOrVisitedSet[fullPath] = struct{}{}

					//todo, handle errors
					workQueue.QueueWork(
						getNextWorkItem(
							folderInfo,
							fullPath,
							workQueue,
							visitedElementsLock,
							hasher,
							errorHandler,
							storeDataFunction,
							queuedOrVisitedSet,
							logger,
						),
					)
					if d.IsDir() {
						returnValue = filepath.SkipDir
					}
				}

				return returnValue
			})
			if err != nil {
				// todo handle this err
				panic(err)
			}
		} else {
			hash, err := hasher.HashFile(pathForHashOrWalk)
			if err != nil {
				errorHandler.HandleError(errorhandling.NewFimulcrumError("problem hashing file", "error", err.Error(), "filepath", pathForHashOrWalk))
			}
			fileName := fileInfo.Name()
			dataToPersist := &persistence.HashData{Name: fileName, NewName: fileName, Path: pathForHashOrWalk, Hash: hash}
			err = storeDataFunction(dataToPersist)
			if err != nil {
				errorHandler.HandleError(errorhandling.NewFimulcrumError("problem storing hash data for file", "error", err.Error(), "filepath", pathForHashOrWalk))
			}
		}
		return nil
	}
}
