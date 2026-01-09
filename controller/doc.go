// Package controller contains the building blocks for creating Kubernetes
// controllers.
package controller

//revive:disable:line-length-limit  // go:generate directives are long.

//go:generate mockgen -package=controller_test -destination=mock_queue_test.go -source=queue.go Queue
//go:generate mockgen -package=controller_test -destination=mock_controller_test.go -source=controller.go Retriever,Handler
//go:generate mockgen -package=controller_test -destination=mock_recorder_test.go -source=recorder.go Recorder
//go:generate mockgen -package=controller_test -destination=mock_watcher_test.go -mock_names=Interface=MockWatcher k8s.io/apimachinery/pkg/watch Interface
//go:generate mockgen -package=controller_test -destination=mock_indexer_test.go k8s.io/client-go/tools/cache Indexer
//go:generate mockgen -package=controller_test -destination=mock_informer_test.go k8s.io/client-go/tools/cache SharedIndexInformer
//go:generate mockgen -package=controller_test -destination=mock_k8sclient_test.go -mock_names=Interface=MockK8sClient k8s.io/client-go/kubernetes Interface
//go:generate mockgen -package=controller_test -destination=mock_k8scorev1_test.go k8s.io/client-go/kubernetes/typed/core/v1 CoreV1Interface
//go:generate mockgen -package=controller_test -destination=mock_k8scoordinationv1_test.go k8s.io/client-go/kubernetes/typed/coordination/v1 CoordinationV1Interface

//revive:enable:line-length-limit
