package main

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"parks-young.com/dffhc/pkg/work"
)

//todo, split worker off to be independent and not care about work type
//todo, move worker code into a pkg file, file operations into a pkg file, storage operations into a package file, and comparison operations into a package file
//todo, rewrite code that does semaphores to be functions so I can defer in them
//todo, deal with error outputs
//todo, deal with queue exhaustion....no more buffer for elements in queue (channel too small....can either dynamically move them over to another channel? probably safest to do once the worker code is more standalone)
//todo, use a logger that will log out text and allow user to specify log level

//todo, I think I want to publish this? https://go.dev/doc/modules/publishing

type HashData struct {
	FileInfo os.FileInfo
	Path     string
	Hash     []byte
}

func (hd *HashData) GetName() (string, error) {
	if hd.FileInfo == nil {
		return "", errors.New("file info missing")
	}
	return hd.FileInfo.Name(), nil
}

type HashStorage interface {
	StoreData(*HashData) error
}

type inMemoryStorage struct {
	files     map[string][]*HashData
	dataMutex sync.Mutex
}

func NewInMemoryStorage() *inMemoryStorage {
	return &inMemoryStorage{
		files:     make(map[string][]*HashData),
		dataMutex: sync.Mutex{},
	}
}

func (ims *inMemoryStorage) StoreData(hd *HashData) error {
	hashAsHexString := fmt.Sprintf("%x", hd.Hash)
	ims.dataMutex.Lock()
	defer ims.dataMutex.Unlock()
	if hd.FileInfo == nil {
		return errors.New("hash data not valid")
	} else if hd.FileInfo.IsDir() {
		return errors.New("data stored must be a file and may not be a directory")
	} else if _, present := ims.files[hashAsHexString]; present {
		ims.files[hashAsHexString] = append(ims.files[hashAsHexString], hd)
		return nil
	} else {
		//not present
		ims.files[hashAsHexString] = []*HashData{hd}
		return nil
	}
}

// todo, make this simple, do the hash, make the comparison a different function
func main() {
	// var quiet bool
	//var continueIfStalled bool
	// var followSymlinks bool //todo not sure if this is relevant anymore

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	dataStorage := NewInMemoryStorage()

	//declare args
	var startPathString string
	var workerThreads int64
	var queueSize int64
	var gatherErrors bool

	// read in args
	flag.StringVar(&startPathString, "path", "", "path to start the hashing")
	flag.Int64Var(&workerThreads, "workers", 2, "number of worker threads, minimum 1")
	flag.Int64Var(&queueSize, "queue", 1024, "size of queue for workers to process, default is 1024, must be at least the same as number of workers")
	flag.BoolVar(&gatherErrors, "gathererrors", false, "true for the application should exit immediately after an error is encountered (such as when trying to read a file), false for continuing and logging information about the error")
	flag.Parse()

	// process args
	cleanedPath, err := filepath.Abs(startPathString)
	if err != nil {
		logger.Error("provided path must be translatable into an absolute path", "path", startPathString, "error", err)
		return
	}
	rootFolderInfo, err := os.Stat(cleanedPath)
	if err != nil {
		logger.Error("unable to validate path", "path", cleanedPath, "error", err)
		return
	} else if !rootFolderInfo.IsDir() {
		logger.Error("path must be a valid directory", "path", cleanedPath, "error", err)
		return
	}

	//todo, how can I prevent a deadlock if queue size is reached? maybe dump output to DB and allow a continuation? Maybe allow offloading of queue into another object or a DB interface?
	workerQueue, err := work.NewEndAtIdleWorkQueue(workerThreads, queueSize, gatherErrors, logger)
	if err != nil {
		logger.Error("issue creating worker queue", "error", err)
		return
	}

	// value of 0 means queued, value of >= 1 means it's been processed at least once
	queuedOrVisitedSet := map[string]struct{}{}
	visitedCheck := sync.Mutex{}

	//add item to work queue and mark it as visited
	queuedOrVisitedSet[cleanedPath] = struct{}{}

	err = workerQueue.QueueWork(
		//todo, rework the order of these args?
		getNextWorkItem(
			&HashData{FileInfo: rootFolderInfo, Path: cleanedPath},
			workerQueue,
			&visitedCheck,
			dataStorage,
			queuedOrVisitedSet,
			logger,
		),
	)
	if err != nil {
		logger.Error("issue queuing work item", "error", err)
		return
	}

	completeCond, err := workerQueue.Start()
	if err != nil {
		logger.Error("issue starting processing of work items", "error", err)
		return
	}
	completeCond.Wait()

	print("hi!")
}

func sha256HashFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

// todo, should create an object for managing searches
func getNextWorkItem(
	data *HashData,
	workQueue work.WorkQueue,
	visitedElementsLock *sync.Mutex,
	dataStorage HashStorage,
	queuedOrVisitedSet map[string]struct{},
	logger *slog.Logger,
) work.WorkItem {
	return func() error {
		isSymlink := data.FileInfo.Mode()&os.ModeSymlink > 0

		if isSymlink {
			//todo need to fix this
			panic("sym links not supported currently")
		} else if data.FileInfo.IsDir() {
			err := filepath.WalkDir(data.Path, func(path string, d fs.DirEntry, err error) error {
				//todo, handle err being passed in as non-nil: https://pkg.go.dev/io/fs#WalkDirFunc

				var returnValue error = nil

				fullPath, err := filepath.Abs(path)
				if err != nil {
					//todo need to handle this err
					panic(err)
				}
				folderInfo, err := os.Stat(fullPath)
				if err != nil {
					//todo need to handle this err
					panic(err)
				}

				visitedElementsLock.Lock()
				defer visitedElementsLock.Unlock()
				if _, visited := queuedOrVisitedSet[fullPath]; !visited {
					queuedOrVisitedSet[fullPath] = struct{}{}

					//todo, handle errors
					err = workQueue.QueueWork(
						getNextWorkItem(
							&HashData{FileInfo: folderInfo, Path: fullPath},
							workQueue,
							visitedElementsLock,
							dataStorage,
							queuedOrVisitedSet,
							logger,
						),
					)
					if err != nil {
						panic(err)
					}
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
			hash, err := sha256HashFile(data.Path)
			if err != nil {
				logger.Error("problem hashing file", "error", err)
				panic("sadfasf") //todo need to handle exit after error here appropriately
			}
			data.Hash = hash
			err = dataStorage.StoreData(data)
			if err != nil {
				//todo, handle this appropriately
				panic("dskjhfakjsdf")
			}
		}
		return nil
	}
}
