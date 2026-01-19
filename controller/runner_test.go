package controller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/mock"
	"github.com/tkrop/go-testing/test"

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
//  1. Find a way to control the detached routine used for the lease renewals,
//  2. Find a way to control the lease and renewal timings, and
//  3. Add improvements to the mock framework to be able to add optional mock
//     calls in a sequence of calls.
//
// NOTE: The resourcelock.New error path (runner.go:108-113) is currently
// unreachable because:
//  1. resourcelock.New only fails for invalid/deprecated lock types
//  2. We hardcode resourcelock.LeasesResourceLock which is valid
//  3. resourcelock.New does NOT validate nil CoreV1/CoordinationV1 clients
//  4. The only way to trigger this would be to pass an invalid lock type,
//     but that would require making the lock type configurable
//
// This represents architectural dead code that provides defensive error
// handling for future API changes.

// CallMockInit sets up the mock for the Init method.

func CallMockInit(err error) mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRunnable).EXPECT().
			Init(gomock.Not(gomock.Nil()), gomock.Not(gomock.Nil())).
			Do(func(_ context.Context, errch chan error) {
				if err != nil {
					errch <- err
				}
			})
	}
}

// CallMockRun sets up the mock for the Run method.
func CallMockRun() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRunnable).EXPECT().
			Run(gomock.Not(gomock.Nil()))
	}
}

// CallMockRunOpt sets up the mock for the Run method.
func CallMockRunOpt() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockRunnable).EXPECT().
			Run(gomock.Not(gomock.Nil())).MaxTimes(1)
	}
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
	RenewPeriod:   20 * time.Millisecond,
}

type defaultRunnerRunParams struct {
	setup     mock.SetupFunc
	runnables int
	expect    error
}

var defaultRunnerRunTestCases = map[string]defaultRunnerRunParams{
	"zero": {
		runnables: 0,
	},
	"single": {
		setup: mock.Chain(
			CallMockInit(nil),
			CallMockRun(),
		),
		runnables: 1,
	},
	"multiple": {
		setup: mock.Chain(
			CallMockInit(nil), CallMockInit(nil), CallMockInit(nil),
			CallMockRun(), CallMockRun(), CallMockRun(),
		),
		runnables: 3,
	},
	"error": {
		setup: mock.Chain(
			CallMockInit(assert.AnError),
			CallMockRunOpt(),
		),
		runnables: 1,
		expect:    assert.AnError,
	},
}

func TestDefaultRunnerRun(t *testing.T) {
	test.Map(t, defaultRunnerRunTestCases).
		Run(func(t test.Test, param defaultRunnerRunParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			runner := controller.NewDefaultRunner()

			runnables := make([]controller.Runnable, 0, param.runnables)
			for range param.runnables {
				runnables = append(runnables, mock.Get(mocks, NewMockRunnable))
			}
			errch := make(chan error, 1)

			// When
			runner.Run(ctx, errch, runnables...)

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
	setup     mock.SetupFunc
	config    *controller.LeaderConfig
	id        string
	runnables int
	call      func(
		runner controller.Runner,
		runnables []controller.Runnable,
		errch chan error,
	)
	expect error
}

var leaderRunnerRunTestCases = map[string]leaderRunnerRunParams{
	"zero-id-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: defaultLeaderConfig,
		expect: controller.ErrController.New(
			"creating elector [name=%s]: %w", "controller",
			//lint:ignore ST1005 // matching Kubernetes error message.
			//nolint:staticcheck // matching Kubernetes error message.
			errors.New("Lock identity is empty"),
		),
	},

	"nil-config-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			test.Panic("runtime error: "+
				"invalid memory address or nil pointer dereference"),
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
		id: "host-1",
		expect: controller.ErrController.
			New("creating elector [name=%s]: %w",
				"invalid-controller", errors.New(
					"leaseDuration must be greater than renewDeadline")),
	},

	"runabe-error": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			CallMockInit(assert.AnError),
			CallMockRunOpt(),
		),
		config: defaultLeaderConfig,
		id:     "host-1", runnables: 1,
		expect: assert.AnError,
	},

	"zero-runnable": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: defaultLeaderConfig,
		id:     "host-zero", runnables: 0,
	},

	"single-runnable": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			CallMockInit(nil),
			CallMockRunOpt(),
		),
		config: defaultLeaderConfig,
		id:     "host-1", runnables: 1,
	},

	"multiple-runnables": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			CallMockInit(nil), CallMockInit(nil), CallMockInit(nil),
			CallMockRun(), CallMockRun(), CallMockRun(),
		),
		config: defaultLeaderConfig,
		id:     "host-1", runnables: 3,
	},

	"minimal-valid-timings": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			CallMockInit(nil),
			CallMockRun(),
		),
		config: &controller.LeaderConfig{
			LeaseDuration: 15 * time.Millisecond,
			RenewDeadline: 10 * time.Millisecond,
			RenewPeriod:   2 * time.Millisecond,
		},
		id: "host-1", runnables: 1,
	},

	"ctx-cancel-before-leadership": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: defaultLeaderConfig,
		id:     "host-cancel-1", runnables: 0,
		call: func(
			runner controller.Runner,
			runnables []controller.Runnable,
			errch chan error,
		) {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()
			runner.Run(cancelCtx, errch, runnables...)
		},
		expect: controller.ErrController.New("running [name=%s]: %w",
			"controller", errors.New("leadership lost")),
	},

	"ctx-cancel-during-leadership": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
			CallMockInit(nil),
			CallMockRun(),
		),
		config: defaultLeaderConfig,
		id:     "host-cancel-2", runnables: 1,
		call: func(
			runner controller.Runner,
			runnables []controller.Runnable,
			errch chan error,
		) {
			cancelCtx, cancel := context.WithCancel(ctx)
			runner.Run(cancelCtx, errch, runnables...)
			time.Sleep(150 * time.Millisecond)
			cancel()
		},
		expect: controller.ErrController.New("running [name=%s]: %w",
			"controller", errors.New("leadership lost")),
	},

	"ctx-timeout": {
		setup: mock.Chain(
			CallK8sClientCoreV1(true),
			CallK8sClientCoordinationV1(),
		),
		config: defaultLeaderConfig,
		id:     "host-timeout", runnables: 0,
		call: func(
			runner controller.Runner,
			runnables []controller.Runnable,
			errch chan error,
		) {
			timeoutCtx, cancel := context.WithTimeout(
				ctx, 50*time.Millisecond)
			defer cancel()
			runner.Run(timeoutCtx, errch, runnables...)
			time.Sleep(100 * time.Millisecond)
		},
		expect: controller.ErrController.New("running [name=%s]: %w",
			"controller", errors.New("leadership lost")),
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

			runnables := make([]controller.Runnable, 0, param.runnables)
			for range param.runnables {
				runnables = append(runnables, mock.Get(mocks, NewMockRunnable))
			}

			errch := make(chan error, 1)

			// When
			if param.call != nil {
				param.call(runner, runnables, errch)
			} else {
				runner.Run(ctx, errch, runnables...)
			}

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
