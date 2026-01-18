package controller_test

import (
	"testing"

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
