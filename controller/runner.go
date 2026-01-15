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

// Default leader election timings.
const (
	// Default leader election lease duration.
	defLeaseDuration = 15 * time.Second
	// Default leader election renew deadline.
	defRenewDeadline = 10 * time.Second
	// Default leader election retry period.
	defRetryPeriod = 2 * time.Second
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
	// RenewDeadline is the duration that the acting master will retry
	// refreshing leadership before giving up.
	RenewDeadline time.Duration `default:"10s"`
	// RetryPeriod is the duration the LeaderElector clients should wait
	// between tries of actions.
	RetryPeriod time.Duration `default:"2s"`
}

// Runner knows how to run the processing queue.
type Runner interface {
	// Run runs the given function with a context as soon as the instance of
	// the runner with the given host identifier takes the lead.
	Run(call func(ctx context.Context), errch chan error)
}

// DefaultRunner is the default implementation of a runner.
type DefaultRunner struct{}

// NewDefaultRunner creates a new default runner that just runs the function.
func NewDefaultRunner() Runner {
	return &DefaultRunner{}
}

// Run runs the given function with a context as soon as the instance of the
// runner with the given host identifier takes the lead.
func (*DefaultRunner) Run(
	call func(ctx context.Context), _ chan error,
) {
	call(context.Background())
}

// LeaderRunner is the leader election default implementation.
type LeaderRunner struct {
	hostid string
	k8scli kubernetes.Interface
	config *LeaderConfig
}

// NewLeaderRunner returns a new leader election runner with given unique host
// identifier that can be used to run a function after acquiring leadership
// using the Kubernetes leader election mechanism. Make sure to use a unique
// host identifier for each instance of a leader eleaction runner, e.g. by
// adding a universal unique identifier to the hostname.
func NewLeaderRunner(
	hostid string,
	config *LeaderConfig,
	k8scli kubernetes.Interface,
) Runner {
	if config == nil {
		config = &LeaderConfig{
			LeaseDuration: defLeaseDuration,
			RenewDeadline: defRenewDeadline,
			RetryPeriod:   defRetryPeriod,
		}
	}

	return &LeaderRunner{
		hostid: hostid,
		config: config,
		k8scli: k8scli,
	}
}

// Run will run the given function with a context as soon as the runner
// instances identified by the given hostid takes the lead.
func (r *LeaderRunner) Run(
	call func(ctx context.Context), errch chan error,
) {
	// Create the resource lock.
	lock, err := resourcelock.New(resourcelock.LeasesResourceLock,
		r.config.Namespace, r.config.Name, r.k8scli.CoreV1(),
		r.k8scli.CoordinationV1(), resourcelock.ResourceLockConfig{
			Identity: r.hostid,
			EventRecorder: record.NewBroadcaster().
				NewRecorder(scheme.Scheme, corev1.EventSource{
					Component: r.config.Name, Host: r.hostid,
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
		RetryPeriod:   r.config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: call,
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
