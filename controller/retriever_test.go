package controller_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-kube/controller"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"
)

type retrieverListParams struct {
	setup  mock.SetupFunc
	expect *List
	error  error
}

var retrieverListTestCases = map[string]retrieverListParams{
	"success": {
		setup:  CallRetrieverList(testList, nil),
		expect: testList,
	},

	"resource-error": {
		setup: CallRetrieverList[*List](nil, assert.AnError),
		error: errTest.New("listing: %w", assert.AnError),
	},
}

func TestRetrieverList(t *testing.T) {
	test.Map(t, retrieverListTestCases).
		Run(func(t test.Test, param retrieverListParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			resource := controller.NewRetriever(
				mock.Get(mocks, NewMockRetriever[*List]), errTest)

			// When
			result, err := resource.List(ctx, testOptions)

			// Then
			assert.Equal(t, param.expect, result)
			assert.Equal(t, param.error, err)
		})
}

type retrieverWatchParams struct {
	setup  mock.SetupFunc
	expect any
	error  error
}

var retrieverWatchTestCases = map[string]retrieverWatchParams{
	"success": {
		setup:  CallRetrieverWatch[*List](nil),
		expect: NewMockWatcher,
	},

	"resource-error": {
		setup:  CallRetrieverWatch[*List](assert.AnError),
		expect: NewMockWatcher,
		error:  errTest.New("watching: %w", assert.AnError),
	},
}

func TestRetrieverWatch(t *testing.T) {
	test.Map(t, retrieverWatchTestCases).
		Run(func(t test.Test, param retrieverWatchParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			resource := controller.NewRetriever(
				mock.Get(mocks, NewMockRetriever[*List]), errTest)
			expect := mocks.GetMock(param.expect)

			// When
			result, err := resource.Watch(ctx, testOptions)

			// Then
			assert.Equal(t, expect, result)
			assert.Equal(t, param.error, err)
		})
}

// CallFilterList expects a list call with the given filter options and error.
func CallFilterList(opts metav1.ListOptions, err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRetriever[*List]).EXPECT().
			List(ctx, opts).Return(testList, err)
	}
}

// CallFilterWatch expects a watch call with the given filter options and error.
func CallFilterWatch(opts metav1.ListOptions, err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRetriever[*List]).EXPECT().
			Watch(ctx, opts).
			Return(mock.Get(mocks, NewMockWatcher), err)
	}
}

type filterRetrieverParams struct {
	label       labels.Selector
	field       fields.Selector
	expect      metav1.ListOptions
	resourceErr error
	listErr     error
	watchErr    error
}

var filterRetrieverTestCases = map[string]filterRetrieverParams{
	"no-selectors": {
		expect: metav1.ListOptions{},
	},

	"label-only": {
		label:  labels.SelectorFromSet(labels.Set{"app": "test"}),
		expect: metav1.ListOptions{LabelSelector: "app=test"},
	},

	"field-only": {
		field:  fields.OneTermEqualSelector("metadata.namespace", "default"),
		expect: metav1.ListOptions{FieldSelector: "metadata.namespace=default"},
	},

	"both-selectors": {
		label: labels.SelectorFromSet(labels.Set{"app": "test"}),
		field: fields.OneTermEqualSelector("metadata.namespace", "default"),
		expect: metav1.ListOptions{
			LabelSelector: "app=test",
			FieldSelector: "metadata.namespace=default",
		},
	},

	"resource-error": {
		resourceErr: assert.AnError,
		listErr:     errTest.New("listing: %w", assert.AnError),
		watchErr:    errTest.New("watching: %w", assert.AnError),
	},
}

func TestFilterRetrieverList(t *testing.T) {
	test.Map(t, filterRetrieverTestCases).
		Run(func(t test.Test, param filterRetrieverParams) {
			// Given
			mocks := mock.NewMocks(t).
				Expect(CallFilterList(param.expect, param.resourceErr))
			resource := controller.NewFilterRetriever(
				mock.Get(mocks, NewMockRetriever[*List]),
				errTest, param.label, param.field)

			// When
			result, err := resource.List(ctx, testOptions)

			// Then
			assert.Equal(t, testList, result)
			assert.Equal(t, param.listErr, err)
		})
}

func TestFilterRetrieverWatch(t *testing.T) {
	test.Map(t, filterRetrieverTestCases).
		Run(func(t test.Test, param filterRetrieverParams) {
			// Given
			mocks := mock.NewMocks(t).
				Expect(CallFilterWatch(param.expect, param.resourceErr))
			resource := controller.NewFilterRetriever(
				mock.Get(mocks, NewMockRetriever[*List]),
				errTest, param.label, param.field)

			// When
			result, err := resource.Watch(ctx, testOptions)

			// Then
			assert.Equal(t, mock.Get(mocks, NewMockWatcher), result)
			assert.Equal(t, param.watchErr, err)
		})
}

func TestNewFilterRetriever(t *testing.T) {
	// Given
	mocks := mock.NewMocks(t)

	// When
	resource := controller.NewFilterRetriever(
		mock.Get(mocks, NewMockRetriever[*List]),
		errTest, labels.Everything(), fields.Everything())

	// Then
	assert.NotNil(t, resource)
}
