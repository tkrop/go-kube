package controller_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/tkrop/go-kube/controller"
	"github.com/tkrop/go-kube/errors"
)

var (
	//nolint:errname // is not an error variable.
	testError   = errors.NewError("test error")
	testObject  = &Object{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	testList    = NewList(testObject)
	testOptions = metav1.ListOptions{LabelSelector: "test=true"}
)

// CallResourceList sets up the mock for the List method.
func CallResourceList(obj **List, err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockResource[*List]).EXPECT().
			List(gomock.Any(), testOptions).Return(obj, err)
	}
}

// CallResourceWatch sets up the mock for the Watch method.
func CallResourceWatch(err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		var watcher watch.Interface
		if err == nil {
			watcher = mock.Get(mocks, NewMockWatcher)
		}

		return mock.Get(mocks, NewMockResource[*List]).EXPECT().
			Watch(ctx, testOptions).Return(watcher, err)
	}
}

func GetMockResourceFunc(
	mocks *mock.Mocks,
) func(namespace string) controller.Resource[*List] {
	return func(_ string) controller.Resource[*List] {
		return mock.Get(mocks, NewMockResource[*List])
	}
}

func GetNilResourceFunc() func(
	namespace string,
) controller.Resource[*List] {
	return func(_ string) controller.Resource[*List] {
		return nil
	}
}

func GetHandleNoop() func(ctx context.Context, obj **List) error {
	return func(_ context.Context, _ **List) error {
		return nil
	}
}

func GetHandleError() func(ctx context.Context, obj **List) error {
	return func(_ context.Context, _ **List) error {
		return assert.AnError
	}
}

func GetHandleValid() func(_ context.Context, obj **List) error {
	return func(_ context.Context, obj **List) error {
		if obj == nil || *obj == nil {
			return assert.AnError
		}

		return nil
	}
}

type newResourceParams struct {
	resource func(namespace string) controller.Resource[*List]
	handle   func(_ context.Context, obj **List) error
	error    *errors.Error
}

var newResourceTestCases = map[string]newResourceParams{
	"success": {
		resource: GetNilResourceFunc(),
		handle:   GetHandleNoop(),
		error:    testError,
	},
}

func TestNewResource(t *testing.T) {
	test.Map(t, newResourceTestCases).
		Run(func(t test.Test, param newResourceParams) {
			// When
			result := controller.NewResource(
				param.resource, param.handle, *param.error)

			// Then
			assert.NotNil(t, result)
		})
}

type resourceListParams struct {
	setup   mock.SetupFunc
	options metav1.ListOptions
	expect  **List
	error   error
}

var resourceListTestCases = map[string]resourceListParams{
	"success": {
		setup:   CallResourceList(&testList, nil),
		options: testOptions,
		expect:  &testList,
	},

	"resource-error": {
		setup:   CallResourceList(nil, assert.AnError),
		options: testOptions,
		error:   testError.New("failed to retrieve: %w", assert.AnError),
	},
}

func TestResourceList(t *testing.T) {
	test.Map(t, resourceListTestCases).
		Run(func(t test.Test, param resourceListParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			resource := controller.NewResource(
				GetMockResourceFunc(mocks),
				GetHandleNoop(), *testError,
			)

			// When
			result, err := resource.List(ctx, param.options)

			// Then
			assert.Equal(t, param.expect, result)
			assert.Equal(t, param.error, err)
		})
}

type resourceWatchParams struct {
	setup   mock.SetupFunc
	options metav1.ListOptions
	expect  any
	error   error
}

var resourceWatchTestCases = map[string]resourceWatchParams{
	"success": {
		setup:   CallResourceWatch(nil),
		options: testOptions,
		expect:  NewMockWatcher,
	},

	"resource-error": {
		setup:   CallResourceWatch(assert.AnError),
		options: testOptions,
		error:   testError.New("failed to watch: %w", assert.AnError),
	},
}

func TestResourceWatch(t *testing.T) {
	test.Map(t, resourceWatchTestCases).
		Run(func(t test.Test, param resourceWatchParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			resource := controller.NewResource(
				GetMockResourceFunc(mocks),
				GetHandleNoop(), *testError,
			)
			expect := mocks.GetMock(param.expect)

			// When
			result, err := resource.Watch(ctx, param.options)

			// Then
			assert.Equal(t, expect, result)
			assert.Equal(t, param.error, err)
		})
}

type resourceHandleParams struct {
	setup  mock.SetupFunc
	obj    runtime.Object
	handle func(ctx context.Context, obj **List) error
	error  error
}

var resourceHandleTestCases = map[string]resourceHandleParams{
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
		error: testError.New("invalid type [type=%T]",
			&Object{ObjectMeta: metav1.ObjectMeta{Name: "wrong"}}),
	},
}

func TestResourceHandle(t *testing.T) {
	test.Map(t, resourceHandleTestCases).
		Run(func(t test.Test, param resourceHandleParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			resource := test.Cast[*controller.ResourceImpl[*List]](
				controller.NewResource(
					GetMockResourceFunc(mocks),
					param.handle, *testError,
				),
			)

			// When
			err := resource.Handle(ctx, param.obj)

			// Then
			assert.Equal(t, param.error, err)
		})
}
