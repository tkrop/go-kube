package controller_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
	"github.com/tkrop/go-kube/errors"
)

// Error instance for testing.
var errTest = errors.New("test")

var (
	testOptions = metav1.ListOptions{LabelSelector: "test=true"}
	testObject  = &Object{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	testList    = NewList(testObject)
)

func GetNilResourceFunc() func(
	namespace string,
) controller.Retriever[*List] {
	return func(_ string) controller.Retriever[*List] {
		return nil
	}
}

func GetHandleNoop() func(ctx context.Context, obj *List) error {
	return func(_ context.Context, _ *List) error {
		return nil
	}
}

func GetHandleError() func(ctx context.Context, obj *List) error {
	return func(_ context.Context, _ *List) error {
		return assert.AnError
	}
}

func GetHandleValid() func(_ context.Context, obj *List) error {
	return func(_ context.Context, obj *List) error {
		if obj == nil {
			return assert.AnError
		}

		return nil
	}
}

type handlerHandleParams struct {
	setup  mock.SetupFunc
	obj    runtime.Object
	handle func(ctx context.Context, obj *List) error
	error  error
}

var handlerHandleTestCases = map[string]handlerHandleParams{
	"success": {
		obj:    testList,
		handle: GetHandleNoop(),
	},

	"success-with-handler": {
		obj:    testList,
		handle: GetHandleValid(),
	},

	"handler-error": {
		obj:    testList,
		handle: GetHandleError(),
		error:  assert.AnError,
	},

	"invalid-type": {
		obj:    &Object{ObjectMeta: metav1.ObjectMeta{Name: "wrong"}},
		handle: GetHandleNoop(),
		error: errTest.New("invalid type [type=%T]",
			&Object{ObjectMeta: metav1.ObjectMeta{Name: "wrong"}}),
	},
}

func TestHandlerHandle(t *testing.T) {
	test.Map(t, handlerHandleTestCases).
		Run(func(t test.Test, param handlerHandleParams) {
			// Given
			mock.NewMocks(t).Expect(param.setup)
			handler := controller.NewHandler(param.handle, errTest)

			// When
			err := handler.Handle(ctx, param.obj)

			// Then
			assert.Equal(t, param.error, err)
		})
}

type handlerNotifyParams struct {
	msg   string
	error error
}

var handlerNotifyTestCases = map[string]handlerNotifyParams{
	"with-error": {
		msg:   "processing failed",
		error: assert.AnError,
	},

	"with-nil-error": {
		msg:   "unexpected nil error",
		error: nil,
	},

	"with-wrapped-error": {
		msg:   "complex error scenario",
		error: errTest.New("wrapped error: %w", assert.AnError),
	},
}

func TestHandlerNotify(t *testing.T) {
	test.Map(t, handlerNotifyTestCases).
		Run(func(_ test.Test, param handlerNotifyParams) {
			// Given
			handler := controller.NewHandler(GetHandleNoop(), errTest)

			// When
			handler.Notify(ctx, param.msg, param.error)

			// Then
		})
}
