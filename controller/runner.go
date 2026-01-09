package controller

import (
	"context"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	// Run runs the given function with a context.
	Run(call func(ctx context.Context) error) error
}

// DefaultRunner is the default implementation of a runner.
type DefaultRunner struct{}

// NewDefaultRunner creates a new default runner that just runs the function.
func NewDefaultRunner() Runner {
	return &DefaultRunner{}
}

// Run will just run the provided function.
func (*DefaultRunner) Run(call func(ctx context.Context) error) error {
	return call(context.Background())
}

// LeaderRunner is the leader election default implementation.
type LeaderRunner struct {
	key       string
	namespace string
	k8scli    kubernetes.Interface
	config    *LeaderConfig
}

// NewLeaderRunner returns a new leader election runner that can be used to
// run a function after acquiring leadership.
func NewLeaderRunner(
	namespace, key string, config *LeaderConfig,
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
		config:    config,
		key:       key,
		namespace: namespace,
		k8scli:    k8scli,
	}
}

// Run will run as soon as the runner instance takes the lead.
func (r *LeaderRunner) Run(call func(ctx context.Context) error) error {
	lock, err := r.InitResourceLock()
	if err != nil {
		return err
	}

	// Channel to get the function returning error.
	errch := make(chan error, 1)

	// The function to execute when start acquired.
	start := func(ctx context.Context) {
		select {
		case <-ctx.Done():
			errch <- nil
		case errch <- call(ctx):
		}
	}
	// The function to execute when leadership is lost.
	stop := func() {
		errch <- ErrController.New("leadership lost")
	}

	// Create the leader election configuration
	config := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: r.config.LeaseDuration,
		RenewDeadline: r.config.RenewDeadline,
		RetryPeriod:   r.config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: start,
			OnStoppedLeading: stop,
		},
	}

	// Create the leader elector.
	elector, err := leaderelection.NewLeaderElector(config)
	if err != nil {
		return ErrController.New("creating elector: %w", err)
	}

	go elector.Run(context.Background())

	// Returns the final result.
	return <-errch
}

// InitResourceLock will initialize the resource lock for leader election.
func (r *LeaderRunner) InitResourceLock() (resourcelock.Interface, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, ErrController.New("getting hostname: %w", err)
	}
	id := hostname + "_" + string(uuid.NewUUID())

	recorder := record.NewBroadcaster().
		NewRecorder(scheme.Scheme, corev1.EventSource{
			Component: r.key, Host: id,
		})

	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		r.namespace, r.key,
		r.k8scli.CoreV1(),
		r.k8scli.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		},
	)
	if err != nil {
		return nil, ErrController.New("creating lock: %w", err)
	}

	return lock, nil
}
