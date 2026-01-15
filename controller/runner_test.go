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

// TestRunnable is a runnable that completes immediately.
type TestRunnable struct{}

// Run implements the Runnable interface.
func (TestRunnable) Run(_ context.Context, _ chan error) {}

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
	RenewPeriod:   20 * time.Millisecond,
}

type defaultRunnerRunParams struct {
	runs   []controller.Runnable
	expect error
}

var defaultRunnerRunTestCases = map[string]defaultRunnerRunParams{
	"single": {
		runs: []controller.Runnable{TestRunnable{}},
	},

	"multiple": {
		runs: []controller.Runnable{TestRunnable{}, TestRunnable{}},
	},
}

func TestDefaultRunnerRun(t *testing.T) {
	test.Map(t, defaultRunnerRunTestCases).
		Run(func(t test.Test, param defaultRunnerRunParams) {
			// Given
			runner := controller.NewDefaultRunner()
			errch := make(chan error, 1)

			// When
			runner.Run(errch, param.runs...)

			// Then
			select {
			case err := <-errch:
				assert.Equal(t, param.expect, err)
			case <-time.After(100 * time.Millisecond):
				assert.Equal(t, param.expect, nil,
					"timed out waiting for error")
			}
		})
}

type leaderRunnerRunParams struct {
	setup  mock.SetupFunc
	config *controller.LeaderConfig
	runs   []controller.Runnable
	id     string
	expect error
}

var leaderRunnerRunTestCases = map[string]leaderRunnerRunParams{
	"success": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: defaultLeaderConfig,
		runs:   []controller.Runnable{TestRunnable{}},
		id:     "host-1",
	},

	"nil-config-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			test.Panic("runtime error: "+
				"invalid memory address or nil pointer dereference"),
		),
		runs: []controller.Runnable{TestRunnable{}},
		id:   "host-1",
	},

	"hostid-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: defaultLeaderConfig,
		runs:   []controller.Runnable{TestRunnable{}},
		id:     "",
		expect: controller.ErrController.New(
			"creating elector [name=%s]: %w", "controller",
			//lint:ignore ST1005 // matching Kubernetes error message.
			//nolint:staticcheck // matching Kubernetes error message.
			errors.New("Lock identity is empty"),
		),
	},

	"elector-config-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: &controller.LeaderConfig{
			Name:          "invalid-controller",
			Namespace:     "default",
			LeaseDuration: 10 * time.Millisecond,
			RenewDeadline: 80 * time.Millisecond,
			RenewPeriod:   20 * time.Millisecond,
		},
		runs: []controller.Runnable{TestRunnable{}},
		id:   "host-1",
		expect: controller.ErrController.
			New("creating elector [name=%s]: %w",
				"invalid-controller", errors.New(
					"leaseDuration must be greater than renewDeadline")),
	},

	"minimal-valid-timings": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1()),
		config: &controller.LeaderConfig{
			LeaseDuration: 15 * time.Millisecond,
			RenewDeadline: 10 * time.Millisecond,
			RenewPeriod:   2 * time.Millisecond,
		},
		runs: []controller.Runnable{TestRunnable{}},
		id:   "host-1",
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
				param.id, param.config, k8scli)

			errch := make(chan error, 1)

			// When
			runner.Run(errch, param.runs...)

			// Then
			select {
			case err := <-errch:
				assert.Equal(t, param.expect, err)
			case <-time.After(200 * time.Millisecond):
				assert.Equal(t, param.expect, nil,
					"timed out waiting for error")
			}
		})
}
