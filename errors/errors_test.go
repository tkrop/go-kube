package errors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tkrop/go-testing/test"

	"github.com/tkrop/go-kube/errors"
)

type notError interface {
	NotErrorMethod()
}

type errorParams struct {
	call func(t test.Test, base error)
}

var errorTestCases = map[string]errorParams{
	"unwrap": {call: func(t test.Test, base error) {
		assert.Equal(t, base, errors.New(base).Unwrap())
	}},
	"is-true": {call: func(t test.Test, base error) {
		assert.True(t, errors.New(base).Is(base))
	}},
	"is-false": {call: func(t test.Test, base error) {
		assert.False(t, errors.New(base).Is(errors.NewError("other")))
	}},
	"as-true": {call: func(t test.Test, base error) {
		assert.True(t, errors.New(base).As(new(error)))
	}},
	"as-false": {call: func(t test.Test, base error) {
		assert.False(t, errors.New(base).As(new(notError)))
	}},
	"error-string": {call: func(t test.Test, base error) {
		assert.Equal(t, "base", errors.New(base).Error())
	}},
	"string-string": {call: func(t test.Test, base error) {
		assert.Equal(t, "base", errors.New(base).String())
	}},
	"new-method": {call: func(t test.Test, base error) {
		err := errors.New(base).New("extra [%s]", "info")
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "base - extra [info]")
	}},
	"wrap-nil": {call: func(t test.Test, base error) {
		err := errors.New(base).Wrap("not wrap", "arg", nil)
		assert.Nil(t, err)
	}},
	"wrap-error": {call: func(t test.Test, base error) {
		err := errors.New(base).Wrap("wrap [%s]: %w", "arg",
			errors.NewError("wrapped"))
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "base - wrap [arg]: wrapped")
	}},
}

func TestErrorMethods(t *testing.T) {
	test.Map(t, errorTestCases).
		Run(func(t test.Test, param errorParams) {
			// Given
			base := errors.NewError("base")

			// When & Then
			param.call(t, base)
		})
}
