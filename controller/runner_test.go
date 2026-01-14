package controller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/tkrop/go-kube/controller"
)

// This is an AI generated test that needs to be improved.
//
// First analysis was conducted on 2026-01-14. Only pain point is the missing
// mock for setup for the leader election lease creation and update testing
// the integration. Mocking those calls in complex due to the detached routine
// used to loop for the lease renewals in combination with the random lease
// and renewal timings added by the leader election logic.
//
// For better testing we need three main improvements:
// 1. Find a way to control the detached routine used for the lease renewals,
// 2. Find a way to control the lease and renewal timings, and
// 3. Add improvements to the mock framework to be able to add optional mock
//    calls in a sequence of calls.

// CallSuccess returns a successful call function.
func CallSuccess(_ context.Context) error {
	return nil
}

// CallError returns an error call function.
func CallError(_ context.Context) error {
	return assert.AnError
}

// CallSlow returns a slow call function.
func CallSlow(_ context.Context) error {
	time.Sleep(50 * time.Millisecond)

	return nil
}

// CallK8sClientCoreV1 sets up the mock for the core methods.
func CallK8sClientCoreV1(core bool) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		if !core {
			return mock.Get(mocks, NewMockK8sClient).EXPECT().CoreV1().
				Return(nil)
		}

		return mock.Get(mocks, NewMockK8sClient).EXPECT().CoreV1().
			Return(mock.Get(mocks, NewMockCoreV1Interface))
	}
}

// CallK8sClientCoordinationV1 sets up the mock for the coordination methods.
func CallK8sClientCoordinationV1() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		coordv1 := fake.NewClientset().CoordinationV1()

		return mock.Get(mocks, NewMockK8sClient).EXPECT().
			CoordinationV1().Return(coordv1)
	}
}

var defaultLeaderConfig = &controller.LeaderConfig{
	Name:          "controller",
	Namespace:     "default",
	LeaseDuration: 100 * time.Millisecond,
	RenewDeadline: 80 * time.Millisecond,
	RetryPeriod:   20 * time.Millisecond,
}

type newDefaultRunnerParams struct{}

var newDefaultRunnerTestCases = map[string]newDefaultRunnerParams{
	"create-default-runner": {},
}

func TestNewDefaultRunner(t *testing.T) {
	test.Map(t, newDefaultRunnerTestCases).
		Run(func(t test.Test, _ newDefaultRunnerParams) {
			// When
			runner := controller.NewDefaultRunner()

			// Then
			assert.NotNil(t, runner)
		})
}

type defaultRunnerRunParams struct {
	call   func(ctx context.Context) error
	expect error
}

var defaultRunnerRunTestCases = map[string]defaultRunnerRunParams{
	"successful-run": {
		call:   CallSuccess,
		expect: nil,
	},

	"run-with-error": {
		call:   CallError,
		expect: assert.AnError,
	},
}

func TestDefaultRunnerRun(t *testing.T) {
	test.Map(t, defaultRunnerRunTestCases).
		Run(func(t test.Test, param defaultRunnerRunParams) {
			// Given
			runner := controller.NewDefaultRunner()

			// When
			err := runner.Run(param.call)

			// Then
			assert.Equal(t, param.expect, err)
		})
}

type newLeaderRunnerParams struct {
	config *controller.LeaderConfig
}

var newLeaderRunnerTestCases = map[string]newLeaderRunnerParams{
	"with-nil-config": {
		config: nil,
	},

	"with-custom-config": {
		config: &controller.LeaderConfig{
			Name:          "custom-controller",
			Namespace:     "kube-system",
			LeaseDuration: 20 * time.Second,
			RenewDeadline: 15 * time.Second,
			RetryPeriod:   5 * time.Second,
		},
	},
}

func TestNewLeaderRunner(t *testing.T) {
	test.Map(t, newLeaderRunnerTestCases).
		Run(func(t test.Test, param newLeaderRunnerParams) {
			// Given
			mocks := mock.NewMocks(t)
			k8scli := mock.Get(mocks, NewMockK8sClient)

			// When
			runner := controller.NewLeaderRunner(
				t.Name(), param.config, k8scli)

			// Then
			assert.NotNil(t, runner)
		})
}

type leaderRunnerRunParams struct {
	setup  mock.SetupFunc
	hostid string
	config *controller.LeaderConfig
	call   func(ctx context.Context) error
	expect error
}

var leaderRunnerRunTestCases = map[string]leaderRunnerRunParams{
	"success": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		hostid: "host-1",
		config: defaultLeaderConfig,
		call:   CallSuccess,
	},

	"success-with-nil-config": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		hostid: "host-1",
		call:   CallSuccess,
	},

	"error-hostid": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		hostid: "",
		config: defaultLeaderConfig,
		call:   CallSuccess,
		expect: controller.ErrController.New("creating elector: %w",
			//lint:ignore ST1005 // matching Kubernetes error message.
			//nolint:staticcheck // matching Kubernetes error message.
			errors.New("Lock identity is empty"),
		),
	},

	"call-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		hostid: "host-1",
		config: defaultLeaderConfig,
		call:   CallError,
		expect: assert.AnError,
	},

	"call-slow": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		hostid: "host-1",
		config: defaultLeaderConfig,
		call:   CallSlow,
	},

	"invalid-elector-config": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		hostid: "host-1",
		config: &controller.LeaderConfig{
			Name:          "invalid-controller",
			Namespace:     "default",
			LeaseDuration: 10 * time.Millisecond,
			RenewDeadline: 80 * time.Millisecond,
			RetryPeriod:   20 * time.Millisecond,
		},
		call: CallSuccess,
		expect: controller.ErrController.
			New("creating elector: %w", errors.New(
				"leaseDuration must be greater than renewDeadline")),
	},

	"minimal-valid-timings": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1()),
		hostid: "host-1",
		config: &controller.LeaderConfig{
			LeaseDuration: 15 * time.Millisecond,
			RenewDeadline: 10 * time.Millisecond,
			RetryPeriod:   2 * time.Millisecond,
		},
		call: CallSuccess,
	},
}

func TestLeaderRunnerRun(t *testing.T) {
	test.Map(t, leaderRunnerRunTestCases).
		// Filter(test.Pattern[leaderRunnerRunParams]("call-slow")).
		Run(func(t test.Test, param leaderRunnerRunParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			k8scli := mock.Get(mocks, NewMockK8sClient)

			runner := controller.NewLeaderRunner(
				param.hostid, param.config, k8scli)

			// When
			err := runner.Run(param.call)

			// Then
			assert.Equal(t, param.expect, err)
		})
}
