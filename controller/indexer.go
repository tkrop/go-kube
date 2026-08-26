package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// IndexOwnerUID is the index of resources by the uids of their owners.
	IndexOwnerUID = "owner-uid"
	// IndexOwnerName is the index of resources by the names of their owners.
	IndexOwnerName = "owner-name"
)

// OwnerIndexers creates the indexers used by `List` to look up the resources
// owned by a given owner without scanning the whole cache.
func OwnerIndexers() cache.Indexers {
	return cache.Indexers{
		IndexOwnerUID:  OwnerUIDIndexFunc,
		IndexOwnerName: OwnerNameIndexFunc,
	}
}

// OwnerUIDIndexFunc indexes the given resource by the uids of its owners.
func OwnerUIDIndexFunc(obj any) ([]string, error) {
	return ownerKeys(obj, func(oref metav1.OwnerReference) string {
		return string(oref.UID)
	})
}

// OwnerNameIndexFunc indexes the given resource by the names of its owners.
func OwnerNameIndexFunc(obj any) ([]string, error) {
	return ownerKeys(obj, func(oref metav1.OwnerReference) string {
		return oref.Name
	})
}

// ownerKeys collects the index keys of the owners of the given resource.
func ownerKeys(
	obj any, key func(metav1.OwnerReference) string,
) ([]string, error) {
	access, ok := obj.(metav1.ObjectMetaAccessor)
	if !ok {
		return nil, ErrController.New("object has no meta: %T", obj)
	}

	orefs := access.GetObjectMeta().GetOwnerReferences()
	keys := make([]string, 0, len(orefs))
	for _, oref := range orefs {
		keys = append(keys, key(oref))
	}

	return keys, nil
}
