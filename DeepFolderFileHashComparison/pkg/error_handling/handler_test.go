package errorhandling_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	errorhandling "parks-young.com/dffhc/pkg/error_handling"
)

func generateSlogAttributesFromStringSlice(args []string) []slog.Attr {
	returnData := []slog.Attr{}
	for indx, item := range args {
		if indx&1 != 0 { //not an even indexed element
			continue
		}

		if indx < len(args)-1 { //there's another element to pull
			returnData = append(
				returnData,
				slog.Attr{
					Key:   item,
					Value: slog.StringValue(args[indx+1]),
				},
			)
		} else {
			returnData = append(
				returnData,
				slog.Attr{
					Key:   "invalidKey",
					Value: slog.StringValue(args[indx]),
				},
			)
		}
	}
	return returnData
}

type caseFormatWithError = struct {
	Args []string
	Msg  string
	Err  error
}

func getStandardTestCasesWithArgsMsgErr(includeNilErrors bool, includeNonNilErrors bool) []caseFormatWithError {
	cases := []caseFormatWithError{}

	errorsToUse := []error{}
	if includeNilErrors {
		errorsToUse = append(errorsToUse, nil)
	}
	if includeNonNilErrors {
		errorsToUse = append(errorsToUse, errors.New("test error"))
	}

	if !includeNilErrors && !includeNonNilErrors {
		panic("getStandardTestCasesWithArgsMsgErr must have includeNilErrors or includeNonNilErrors as true")
	}

	for _, err := range errorsToUse {
		for _, msgString := range []string{"", "test_message"} {
			for _, constructorArgs := range [][]string{
				{},
				{"abcdefg"},
				{"abcdefg", "hijklmnop"},
				{"abcdefg", "hijklmnop", "qrstuv"},
				{"abcdefg", "hijklmnop", "qrstuv", "wxyz"},
			} {
				cases = append(
					cases,
					caseFormatWithError{
						Args: constructorArgs,
						Msg:  msgString,
						Err:  err,
					},
				)
			}
		}
	}
	return cases
}

func TestFimulcrumErrorMessageGetter(t *testing.T) {
	cases := []struct {
		args           []string
		msg            string
		expectedOutput string
	}{}
	for _, msgString := range []string{"", "test_message"} {
		for _, constructorArgs := range [][]string{
			{},
			{"abcdefg"},
			{"abcdefg", "hijklmnop"},
			{"abcdefg", "hijklmnop", "qrstuv"},
			{"abcdefg", "hijklmnop", "qrstuv", "wxyz"},
		} {
			cases = append(
				cases,
				struct {
					args           []string
					msg            string
					expectedOutput string
				}{
					args:           constructorArgs,
					msg:            msgString,
					expectedOutput: msgString,
				},
			)
		}
	}

	for _, testCase := range cases {
		t.Run(fmt.Sprintf("message: %s, number of args: %d", testCase.msg, len(testCase.args)), func(t *testing.T) {
			uut := errorhandling.FimulcrumErrorFromError(nil, testCase.msg, testCase.args...)
			result := uut.Message()
			if result != testCase.expectedOutput {
				t.Fatalf("%s not equal to expected value of %s", result, testCase.expectedOutput)
			}
		})
	}
}

func TestFimulcrumErrorMessageGetterWithZeroValue(t *testing.T) {
	expectedValue := ""
	uut := errorhandling.FimulcrumError{}
	result := uut.Message()
	if result != expectedValue {
		t.Fatalf("%s not equal to expected value of %s", result, expectedValue)
	}
}

func TestFimulcrumErrorErrorGetter(t *testing.T) {
	cases := []struct {
		args           []string
		msg            string
		err            error
		expectedOutput string
	}{
		{
			[]string{}, "", nil, "FimulcrumError",
		},
		{
			[]string{}, "test_message", nil, "FimulcrumError, test_message",
		},
		{
			[]string{"abcdefg"}, "", nil, "FimulcrumError: invalidKey=abcdefg",
		},
		{
			[]string{"abcdefg"}, "test_message", nil, "FimulcrumError, test_message: invalidKey=abcdefg",
		},
		{
			[]string{"abcdefg", "hijklmnop"}, "", nil, "FimulcrumError: abcdefg=hijklmnop",
		},
		{
			[]string{"abcdefg", "hijklmnop"}, "test_message", nil, "FimulcrumError, test_message: abcdefg=hijklmnop",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv"}, "", nil, "FimulcrumError: abcdefg=hijklmnop;invalidKey=qrstuv",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv"}, "test_message", nil, "FimulcrumError, test_message: abcdefg=hijklmnop;invalidKey=qrstuv",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv", "wxyz"}, "", nil, "FimulcrumError: abcdefg=hijklmnop;qrstuv=wxyz",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv", "wxyz"}, "test_message", nil, "FimulcrumError, test_message: abcdefg=hijklmnop;qrstuv=wxyz",
		},
		{
			[]string{}, "", errors.New("test error"), "FimulcrumError, test error",
		},
		{
			[]string{}, "test_message", errors.New("test error"), "FimulcrumError, test error, test_message",
		},
		{
			[]string{"abcdefg"}, "", errors.New("test error"), "FimulcrumError, test error: invalidKey=abcdefg",
		},
		{
			[]string{"abcdefg"}, "test_message", errors.New("test error"), "FimulcrumError, test error, test_message: invalidKey=abcdefg",
		},
		{
			[]string{"abcdefg", "hijklmnop"}, "", errors.New("test error"), "FimulcrumError, test error: abcdefg=hijklmnop",
		},
		{
			[]string{"abcdefg", "hijklmnop"}, "test_message", errors.New("test error"), "FimulcrumError, test error, test_message: abcdefg=hijklmnop",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv"}, "", errors.New("test error"), "FimulcrumError, test error: abcdefg=hijklmnop;invalidKey=qrstuv",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv"}, "test_message", errors.New("test error"), "FimulcrumError, test error, test_message: abcdefg=hijklmnop;invalidKey=qrstuv",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv", "wxyz"}, "", errors.New("test error"), "FimulcrumError, test error: abcdefg=hijklmnop;qrstuv=wxyz",
		},
		{
			[]string{"abcdefg", "hijklmnop", "qrstuv", "wxyz"}, "test_message", errors.New("test error"), "FimulcrumError, test error, test_message: abcdefg=hijklmnop;qrstuv=wxyz",
		},
	}

	for _, testCase := range cases {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.msg, len(testCase.args), testCase.err), func(t *testing.T) {
			uut := errorhandling.FimulcrumErrorFromError(testCase.err, testCase.msg, testCase.args...)
			result := uut.Error()
			if result != testCase.expectedOutput {
				t.Fatalf("%s not equal to expected value of %s", result, testCase.expectedOutput)
			}
		})
	}
}

func TestFimulcrumErrorErrorGetterWithZeroValue(t *testing.T) {
	expectedValue := "FimulcrumError"
	uut := errorhandling.FimulcrumError{}
	result := uut.Error()
	if result != expectedValue {
		t.Fatalf("%s not equal to expected value of %s", result, expectedValue)
	}
}

func TestFilmulcrumErrorUnwrap(t *testing.T) {
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(true, true) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			uut := errorhandling.FimulcrumErrorFromError(testCase.Err, testCase.Msg, testCase.Args...)
			result := uut.Unwrap()
			if result != testCase.Err {
				t.Fatalf("%s not equal to expected value of %s", result, testCase.Err)
			}
		})
	}
}

func TestFilmulcrumErrorUnwrapWithZeroValue(t *testing.T) {
	uut := errorhandling.FimulcrumError{}
	result := uut.Unwrap()
	if result != nil {
		t.Fatalf("%s not equal to expected value of nil", result)
	}
}

func TestFilmulcrumErrorIsErrorWorks(t *testing.T) {
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(false, true) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			uut := errorhandling.FimulcrumErrorFromError(testCase.Err, testCase.Msg, testCase.Args...)
			if !errors.Is(uut, testCase.Err) {
				t.Fatal("error Is call did not return true")
			}
		})
	}
}

type OtherError struct {
	InnerError error
}

func (*OtherError) Error() string {
	return "this is an error used for testing purposes only"
}

func (oe *OtherError) Unwrap() error {
	return oe.InnerError
}

type OtherOtherError struct {
	InnerError error
}

func (*OtherOtherError) Error() string {
	return "this is an error used for testing purposes only, making sure Fimulcrum errors act appropriately"
}

func (oe *OtherOtherError) Unwrap() error {
	return oe.InnerError
}

func TestFilmulcrumErrorAsWithFimulcrumOuterError(t *testing.T) {
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(true, true) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			errorInsideUut := &OtherError{InnerError: testCase.Err}
			uut := errorhandling.FimulcrumErrorFromError(errorInsideUut, testCase.Msg, testCase.Args...)
			var other *OtherError
			if !errors.As(uut, &other) {
				t.Fatal("error As call did not return true")
			} else if errorInsideUut != other {
				t.Fatal("value from As call not expected")
			}
		})
	}
}

func TestFilmulcrumErrorAsWithFimulcrumOuterErrorDoesNotConstructRandomOtherType(t *testing.T) {
	//this is probably overkill, but ¯\_(ツ)_/¯
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(true, true) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			errorInsideUut := &OtherError{InnerError: testCase.Err}
			uut := errorhandling.FimulcrumErrorFromError(errorInsideUut, testCase.Msg, testCase.Args...)
			var other *OtherOtherError
			if errors.As(uut, &other) {
				t.Fatal("error As call succeeded when it wasn't expected to")
			}
		})
	}
}

func TestFilmulcrumErrorAsWithFimulcrumInnerErrorWorks(t *testing.T) {
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(true, true) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			uut := errorhandling.FimulcrumErrorFromError(testCase.Err, testCase.Msg, testCase.Args...)
			containingError := OtherError{InnerError: uut}
			var uutMatch *errorhandling.FimulcrumError

			if !errors.As(&containingError, &uutMatch) {
				t.Fatal("error Is call did not return true")
			} else if uutMatch != uut {
				t.Fatal("value from As call not expected")
			}
		})
	}
}

func TestFilmulcrumErrorGetArgsForLog(t *testing.T) {
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(true, true) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			uut := errorhandling.FimulcrumErrorFromError(testCase.Err, testCase.Msg, testCase.Args...)
			expectedResult := generateSlogAttributesFromStringSlice(testCase.Args)
			result := uut.GetArgsForLog()

			errorMessage := fmt.Sprintf("expected and actual do not match: %s, %s", expectedResult, result)

			if len(result) != len(expectedResult) {
				t.Fatal(errorMessage)
			}

			for i := 0; i < len(expectedResult); i++ {
				if !result[i].Equal(expectedResult[i]) {
					t.Fatal(errorMessage)
				}
			}

		})
	}
}

func TestFilmulcrumErrorConstructorsEquivalent(t *testing.T) {
	for _, testCase := range getStandardTestCasesWithArgsMsgErr(true, false) {
		t.Run(fmt.Sprintf("message: %s, number of args: %d, error: %s", testCase.Msg, len(testCase.Args), testCase.Err), func(t *testing.T) {
			uutOne := errorhandling.FimulcrumErrorFromError(testCase.Err, testCase.Msg, testCase.Args...)
			uutTwo := errorhandling.NewFimulcrumError(testCase.Msg, testCase.Args...)
			oneAsString := fmt.Sprintf("%#v", uutOne)
			twoAsString := fmt.Sprintf("%#v", uutTwo)

			//indirectly using reflection and just doing a string comparison to make sure these objects are the same
			if oneAsString != twoAsString {
				t.Fatalf("two versions of constructor don't seem to produce identical objects: %s != %s", oneAsString, twoAsString)
			}
		})
	}
}
