package errorhandling

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

type FimulcrumError struct {
	message     string
	args        []slog.Attr
	parentError error
}

func NewFimulcrumError(message string, args ...string) *FimulcrumError {
	return FimulcrumErrorFromError(nil, message, args...)
}

func FimulcrumErrorFromError(err error, message string, args ...string) *FimulcrumError {
	hasOddNumberOfElements := (len(args) & 1) > 0
	sizeOfAttrs := len(args) / 2 //be careful, this is integer division
	if hasOddNumberOfElements {
		sizeOfAttrs += 1
	}

	fErr := FimulcrumError{
		message:     message,
		args:        make([]slog.Attr, sizeOfAttrs),
		parentError: err,
	}

	for index, item := range args {
		if index&1 > 0 {
			continue //skip elements on an odd index because we're handling them
		}

		var key string
		var value slog.Value
		if index < len(args)-1 {
			key = item
			value = slog.StringValue(args[index+1])
		} else {
			key = "invalidKey"
			value = slog.StringValue(item)
		}

		fErr.args[index/2] = slog.Attr{
			Key:   key,
			Value: value,
		}
	}

	return &fErr
}

func (fe *FimulcrumError) Error() string {
	valsAsStrings := make([]string, len(fe.args))
	for ind, item := range fe.args {
		valsAsStrings[ind] = item.String()
	}

	//it's possible the strings this delimits will have the same char in them, we don't really expect users to do a split
	//on the strings, so, if it does, meh
	delimiter := ";"

	if fe.parentError != nil && len(fe.args) > 0 && len(fe.message) > 0 {
		return fmt.Sprintf("FimulcrumError, %s, %s: %s", fe.parentError, fe.message, strings.Join(valsAsStrings, delimiter))
	} else if fe.parentError != nil && len(fe.args) > 0 && len(fe.message) == 0 {
		return fmt.Sprintf("FimulcrumError, %s: %s", fe.parentError, strings.Join(valsAsStrings, delimiter))
	} else if fe.parentError != nil && len(fe.args) == 0 && len(fe.message) > 0 {
		return fmt.Sprintf("FimulcrumError, %s, %s", fe.parentError, fe.message)
	} else if fe.parentError != nil && len(fe.args) == 0 && len(fe.message) == 0 {
		return fmt.Sprintf("FimulcrumError, %s", fe.parentError)
	} else if fe.parentError == nil && len(fe.args) > 0 && len(fe.message) > 0 {
		return fmt.Sprintf("FimulcrumError, %s: %s", fe.message, strings.Join(valsAsStrings, delimiter))
	} else if fe.parentError == nil && len(fe.args) > 0 && len(fe.message) == 0 {
		return fmt.Sprintf("FimulcrumError: %s", strings.Join(valsAsStrings, delimiter))
	} else if fe.parentError == nil && len(fe.args) == 0 && len(fe.message) > 0 {
		return fmt.Sprintf("FimulcrumError, %s", fe.message)
	} else { //fe.parentError == nil && len(fe.args) == 0 && len(fe.message) == 0
		return "FimulcrumError"
	}
}

func (fe *FimulcrumError) Unwrap() error {
	return fe.parentError
}

func (fe *FimulcrumError) Message() string {
	return fe.message
}

func (fe *FimulcrumError) GetArgsForLog() []slog.Attr {
	argsSlice := make([]slog.Attr, len(fe.args))
	for indx, arg := range fe.args {
		argsSlice[indx] = slog.Attr{
			Key:   arg.Key,
			Value: arg.Value,
		}
	}
	return argsSlice
}

type ErrorHandler interface {
	HandleError(errorToHandle *FimulcrumError)
}

type LogOrPanicErrorHandler struct {
	panicOnError     bool
	logger           *slog.Logger
	errorDetails     []FimulcrumError
	logOnErrorHandle bool
}

func NewLogOrPanicErrorHandler(panicOnError bool, logger *slog.Logger, logOnErrorHandle bool) (*LogOrPanicErrorHandler, error) {
	if logger == nil && logOnErrorHandle {
		return nil, errors.New("LogOrPanicErrorHandler must have a logger set (not nil) when logOnErrorHandle is true")
	}

	return &LogOrPanicErrorHandler{
		panicOnError:     panicOnError,
		logger:           logger,
		errorDetails:     make([]FimulcrumError, 0),
		logOnErrorHandle: logOnErrorHandle,
	}, nil
}

func (lopeh *LogOrPanicErrorHandler) HandleError(errorToHandle *FimulcrumError) {
	// logger not allowed to be nil if this is true, so just go for it
	if lopeh.logOnErrorHandle {
		argsCopy := errorToHandle.GetArgsForLog()
		argsAsAny := make([]any, 2*len(argsCopy))
		for indx, arg := range argsCopy {
			argsAsAny[indx*2] = arg.Key
			argsAsAny[indx*2+1] = arg.Value
		}

		lopeh.logger.Error(errorToHandle.Message(), argsAsAny...)
	}

	if lopeh.panicOnError {
		panic(errorToHandle.Error())
	}
}

func (lopeh LogOrPanicErrorHandler) GetErrors() []FimulcrumError {
	var errorCopy []FimulcrumError
	copy(errorCopy, lopeh.errorDetails)

	return errorCopy
}
