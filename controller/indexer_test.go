package controller_test

import (
	"testing"

	"k8s.io/client-go/tools/cache"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
)

type ownerIndexersParams struct {
	expect []string
}

var ownerIndexersTestCases = map[string]ownerIndexersParams{
	"owner-indexers": {
		expect: []string{
			controller.IndexOwnerName, controller.IndexOwnerUID,
		},
	},
}

func TestOwnerIndexers(t *testing.T) {
	test.Map(t, ownerIndexersTestCases).
		Run(func(t test.Test, param ownerIndexersParams) {
			// When
			indexers := controller.OwnerIndexers()

			// Then
			names := make([]string, 0, len(indexers))
			for name := range indexers {
				names = append(names, name)
			}
			assert.ElementsMatch(t, param.expect, names)
		})
}

type ownerIndexParams struct {
	index  cache.IndexFunc
	obj    any
	expect []string
	error  error
}

var ownerIndexTestCases = map[string]ownerIndexParams{
	"uid-no-owner": {
		index:  controller.OwnerUIDIndexFunc,
		obj:    p1,
		expect: []string{},
	},

	"uid-single-owner": {
		index:  controller.OwnerUIDIndexFunc,
		obj:    p2,
		expect: []string{"owner-id"},
	},

	"uid-no-meta": {
		index: controller.OwnerUIDIndexFunc,
		obj:   "no-meta",
		error: controller.ErrController.New(
			"object has no meta: %T", "no-meta"),
	},

	"name-no-owner": {
		index:  controller.OwnerNameIndexFunc,
		obj:    p1,
		expect: []string{},
	},

	"name-single-owner": {
		index:  controller.OwnerNameIndexFunc,
		obj:    p2,
		expect: []string{"owner"},
	},

	"name-no-meta": {
		index: controller.OwnerNameIndexFunc,
		obj:   nil,
		error: controller.ErrController.New(
			"object has no meta: %T", nil),
	},
}

func TestOwnerIndex(t *testing.T) {
	test.Map(t, ownerIndexTestCases).
		Run(func(t test.Test, param ownerIndexParams) {
			// When
			keys, err := param.index(param.obj)

			// Then
			assert.Equal(t, param.expect, keys)
			assert.Equal(t, param.error, err)
		})
}
