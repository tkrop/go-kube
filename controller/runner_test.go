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

// TODO: this is an AI generated test that needs to be reviewed and improved.

func CallK8sClientCoreV1() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		corev1 := fake.NewClientset().CoreV1()

		return mock.Get(mocks, NewMockK8sClient).EXPECT().
			CoreV1().Return(corev1).AnyTimes()
	}
}

func CallK8sClientCoordinationV1() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		coordv1 := fake.NewClientset().CoordinationV1()

		return mock.Get(mocks, NewMockK8sClient).EXPECT().
			CoordinationV1().Return(coordv1).AnyTimes()
	}
}

func CallSetupK8sClient() mock.SetupFunc {
	return mock.Chain(
		CallK8sClientCoreV1(),
		CallK8sClientCoordinationV1())
}

func CallK8sClientCoreV1Nil() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockK8sClient).EXPECT().
			CoreV1().Return(nil).AnyTimes()
	}
}

func CallK8sClientCoordinationV1Nil() mock.SetupFunc {
	return func(mocks *mock.Mocks) any {
		return mock.Get(mocks, NewMockK8sClient).EXPECT().
			CoordinationV1().Return(nil).AnyTimes()
	}
}

func CallSetupK8sClientCoreV1Nil() mock.SetupFunc {
	return mock.Chain(
		CallK8sClientCoreV1Nil(),
		CallK8sClientCoordinationV1())
}

func CallSetupK8sClientCoordinationV1Nil() mock.SetupFunc {
	return mock.Chain(
		CallK8sClientCoreV1(),
		CallK8sClientCoordinationV1Nil())
}

var defaultLeaderConfig = &controller.LeaderConfig{
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
			// Given

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
		call: func(_ context.Context) error {
			return nil
		},
		expect: nil,
	},

	"run-with-error": {
		call: func(_ context.Context) error {
			return assert.AnError
		},
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
	key       string
	namespace string
	config    *controller.LeaderConfig
}

var newLeaderRunnerTestCases = map[string]newLeaderRunnerParams{
	"with-nil-config": {
		key:       "test-controller",
		namespace: "default",
		config:    nil,
	},

	"with-custom-config": {
		key:       "custom-controller",
		namespace: "kube-system",
		config: &controller.LeaderConfig{
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
			k8scli := fake.NewClientset()

			// When
			runner := controller.NewLeaderRunner(
				param.namespace, param.key, param.config, k8scli)

			// Then
			assert.NotNil(t, runner)
		})
}

type leaderRunnerRunParams struct {
	setup  mock.SetupFunc
	config *controller.LeaderConfig
	call   func(ctx context.Context) error
	expect error
}

var leaderRunnerRunTestCases = map[string]leaderRunnerRunParams{
	"success": {
		setup:  CallSetupK8sClient(),
		config: defaultLeaderConfig,
		call: func(_ context.Context) error {
			return nil
		},
	},

	"success-with-nil-config": {
		setup: CallSetupK8sClient(),
		call: func(_ context.Context) error {
			return nil
		},
	},

	"call-error": {
		setup:  CallSetupK8sClient(),
		config: defaultLeaderConfig,
		call: func(_ context.Context) error {
			return errors.New("processing error")
		},
		expect: errors.New("processing error"),
	},

	"call-slow": {
		setup:  CallSetupK8sClient(),
		config: defaultLeaderConfig,
		call: func(_ context.Context) error {
			time.Sleep(50 * time.Millisecond)

			return nil
		},
	},

	"invalid-elector-config": {
		setup: CallSetupK8sClient(),
		config: &controller.LeaderConfig{
			LeaseDuration: 10 * time.Millisecond,
			RenewDeadline: 80 * time.Millisecond,
			RetryPeriod:   20 * time.Millisecond,
		},
		call: func(_ context.Context) error {
			return nil
		},
		expect: controller.ErrController.
			New("creating elector: %w", errors.New(
				"leaseDuration must be greater than renewDeadline")),
	},

	"minimal-valid-timings": {
		setup: CallSetupK8sClient(),
		config: &controller.LeaderConfig{
			LeaseDuration: 15 * time.Millisecond,
			RenewDeadline: 10 * time.Millisecond,
			RetryPeriod:   2 * time.Millisecond,
		},
		call: func(_ context.Context) error {
			return nil
		},
	},
}

func TestLeaderRunnerRun(t *testing.T) {
	test.Map(t, leaderRunnerRunTestCases).
		Run(func(t test.Test, param leaderRunnerRunParams) {
			// Given
			mocks := mock.NewMocks(t).Expect(param.setup)
			k8scli := mock.Get(mocks, NewMockK8sClient)

			runner := controller.NewLeaderRunner(
				"default", "run", param.config, k8scli)

			// When
			err := runner.Run(param.call)

			// Then
			assert.Equal(t, param.expect, err)
		})
}
