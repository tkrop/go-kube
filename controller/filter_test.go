package controller_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/controller"
)

// Pass creates a filter returning the given result.
func Pass(result bool) controller.Filter {
	return func(controller.Op, runtime.Object, runtime.Object) bool {
		return result
	}
}

// Filters wraps the given filter into a variadic filter list dropping nil.
func Filters(filter controller.Filter) []controller.Filter {
	if filter == nil {
		return nil
	}

	return []controller.Filter{filter}
}

// NewPod creates a pod with the given generation and resource version.
func NewPod(generation int64, version string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "pod",
		Generation: generation, ResourceVersion: version,
	}}
}

type opStringParams struct {
	op     controller.Op
	expect string
}

var opStringTestCases = map[string]opStringParams{
	"add":     {op: controller.OpAdd, expect: "add"},
	"update":  {op: controller.OpUpdate, expect: "update"},
	"delete":  {op: controller.OpDelete, expect: "delete"},
	"unknown": {op: controller.Op(-1), expect: "unknown"},
}

func TestOpString(t *testing.T) {
	test.Map(t, opStringTestCases).
		Run(func(t test.Test, param opStringParams) {
			// When
			result := param.op.String()

			// Then
			assert.Equal(t, param.expect, result)
		})
}

type filterParams struct {
	filter controller.Filter
	op     controller.Op
	prev   runtime.Object
	obj    runtime.Object
	expect bool
}

var filterTestCases = map[string]filterParams{
	// Generation filter.
	"generation-changed": {
		filter: controller.GenerationChanged(),
		op:     controller.OpUpdate,
		prev:   NewPod(1, "100"),
		obj:    NewPod(2, "101"),
		expect: true,
	},

	"generation-unchanged": {
		filter: controller.GenerationChanged(),
		op:     controller.OpUpdate,
		prev:   NewPod(1, "100"),
		obj:    NewPod(1, "101"),
	},

	"generation-on-add": {
		filter: controller.GenerationChanged(),
		op:     controller.OpAdd,
		obj:    NewPod(1, "100"),
		expect: true,
	},

	"generation-on-delete": {
		filter: controller.GenerationChanged(),
		op:     controller.OpDelete,
		obj:    NewPod(1, "100"),
		expect: true,
	},

	"generation-no-prev-meta": {
		filter: controller.GenerationChanged(),
		op:     controller.OpUpdate,
		obj:    NewPod(1, "100"),
		expect: true,
	},

	"generation-no-obj-meta": {
		filter: controller.GenerationChanged(),
		op:     controller.OpUpdate,
		prev:   NewPod(1, "100"),
		expect: true,
	},

	// Resource version filter.
	"version-changed": {
		filter: controller.ResourceVersionChanged(),
		op:     controller.OpUpdate,
		prev:   NewPod(1, "100"),
		obj:    NewPod(1, "101"),
		expect: true,
	},

	"version-unchanged": {
		filter: controller.ResourceVersionChanged(),
		op:     controller.OpUpdate,
		prev:   NewPod(1, "100"),
		obj:    NewPod(1, "100"),
	},

	"version-on-add": {
		filter: controller.ResourceVersionChanged(),
		op:     controller.OpAdd,
		obj:    NewPod(1, "100"),
		expect: true,
	},

	// Combinators.
	"and-empty": {
		filter: controller.And(),
		expect: true,
	},

	"and-all-true": {
		filter: controller.And(Pass(true), Pass(true)),
		expect: true,
	},

	"and-one-false": {
		filter: controller.And(Pass(true), Pass(false)),
	},

	"or-empty": {
		filter: controller.Or(),
	},

	"or-one-true": {
		filter: controller.Or(Pass(false), Pass(true)),
		expect: true,
	},

	"or-all-false": {
		filter: controller.Or(Pass(false), Pass(false)),
	},

	"not-true": {
		filter: controller.Not(Pass(true)),
	},

	"not-false": {
		filter: controller.Not(Pass(false)),
		expect: true,
	},

	// Composition of the shipped predicates.
	"and-generation-version": {
		filter: controller.And(
			controller.GenerationChanged(),
			controller.ResourceVersionChanged(),
		),
		op:     controller.OpUpdate,
		prev:   NewPod(1, "100"),
		obj:    NewPod(2, "101"),
		expect: true,
	},

	"and-generation-version-resync": {
		filter: controller.And(
			controller.GenerationChanged(),
			controller.ResourceVersionChanged(),
		),
		op:   controller.OpUpdate,
		prev: NewPod(1, "100"),
		obj:  NewPod(1, "100"),
	},
}

func TestFilter(t *testing.T) {
	test.Map(t, filterTestCases).
		Run(func(t test.Test, param filterParams) {
			// When
			result := param.filter(param.op, param.prev, param.obj)

			// Then
			assert.Equal(t, param.expect, result)
		})
}
