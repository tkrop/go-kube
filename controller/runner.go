package controller

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

// LeaderConfig is the configuration for the leader election, i.e. lease
// duration, renew deadline, and retry period.
type LeaderConfig struct {
	// Name is the basic name of the resource lock.
	Name string `default:"controller"`
	// Namespace is the namespace of the resource lock.
	Namespace string `default:"default"`

	// LeaseDuration is the duration that non-leader candidates will
	// wait to force acquire leadership. This is measured against time of
	// last observed ack.
	LeaseDuration time.Duration `default:"15s"`
	// RenewDeadline is the duration that the acting leader will retry
	// refreshing leadership before giving up.
	RenewDeadline time.Duration `default:"10s"`
	// RenewPeriod is the duration the leader elector clients should wait
	// between tries of actions.
	RenewPeriod time.Duration `default:"2s"`
}

type Runnable interface {
	// Run runs the runnable using the given context and error channel for
	// reporting errors.
	Run(ctx context.Context, errch chan error)
}

// Runner knows how to run the processing queue.
type Runner interface {
	// Run runs the given controllers using the given error channel for
	// reporting errors.
	Run(errch chan error, runnables ...Runnable)
}

// DefaultRunner is the default implementation of a runner.
type DefaultRunner struct{}

// NewDefaultRunner creates a new default runner that just runs the function.
func NewDefaultRunner() Runner {
	return &DefaultRunner{}
}

// Run runs the given controllers using the given error channel for reporting
// errors.
func (*DefaultRunner) Run(errch chan error, runnables ...Runnable) {
	for _, run := range runnables {
		go run.Run(context.Background(), errch)
	}
}

// LeaderRunner is the leader election default implementation.
type LeaderRunner struct {
	// Unique identifier of the runner instance.
	id string
	// Leader election configuration.
	config *LeaderConfig
	// Kubernetes client.
	k8scli kubernetes.Interface
}

// NewLeaderRunner returns a new leader election runner with given unique host
// identifier that can be used to run a function after acquiring leadership
// using the Kubernetes leader election mechanism. Make sure to use a unique
// host identifier for each instance of a leader eleaction runner, e.g. by
// adding a universal unique identifier to the hostname.
func NewLeaderRunner(
	id string, config *LeaderConfig, k8scli kubernetes.Interface,
) Runner {
	return &LeaderRunner{
		id:     id,
		config: config,
		k8scli: k8scli,
	}
}

// Run runs the given controllers using leader election using the given error
// channel for reporting errors.
func (r *LeaderRunner) Run(errch chan error, runnables ...Runnable) {
	// Create the resource lock.
	lock, err := resourcelock.New(resourcelock.LeasesResourceLock,
		r.config.Namespace, r.config.Name, r.k8scli.CoreV1(),
		r.k8scli.CoordinationV1(), resourcelock.ResourceLockConfig{
			Identity: r.id,
			EventRecorder: record.NewBroadcaster().
				NewRecorder(scheme.Scheme, corev1.EventSource{
					Component: r.config.Name, Host: r.id,
				}),
		})
	if err != nil {
		errch <- ErrController.Wrap("creating lock [name=%s]: %w",
			r.config.Name, err)

		return
	}

	// Create the leader election configuration.
	config := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: r.config.LeaseDuration,
		RenewDeadline: r.config.RenewDeadline,
		RetryPeriod:   r.config.RenewPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				for _, runnable := range runnables {
					go runnable.Run(ctx, errch)
				}
			},
			OnStoppedLeading: func() {
				errch <- ErrController.New("running [name=%s]: %w",
					r.config.Name, errors.New("leadership lost"))
			},
		},
	}

	// Create the leader elector.
	elector, err := leaderelection.NewLeaderElector(config)
	if err != nil {
		errch <- ErrController.New("creating elector [name=%s]: %w",
			r.config.Name, err)

		return
	}

	go elector.Run(context.Background())
}
