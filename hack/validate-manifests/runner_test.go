package main

import (
	"fmt"
	"io"
	"testing"

	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/stretchr/testify/assert"
)

func TestRunner(t *testing.T) {
	assert := assert.New(t)

	ctx := NewContext(output.NewNonInteractiveShell(io.Discard, io.Discard, 0), DefaultConfig())
	runner := &Runner{}
	check1 := &mockValidator{Turns: 1}

	runner.AddCheck(check1)
	assert.Equal(0, check1.counter)
	assert.False(check1.done)

	complete := runner.Run(ctx)
	assert.Equal(1, check1.counter)
	assert.True(check1.done)
	assert.True(complete)

	complete = runner.Run(ctx)
	assert.Equal(1, check1.counter)
	assert.True(check1.done)
	assert.True(complete)

	check1 = &mockValidator{Turns: 1}
	check2 := &mockValidator{Turns: 2}
	check3 := &mockValidator{Turns: 3}
	runner.AddCheck(check1)
	runner.AddCheck(check2)
	runner.AddCheck(check3)
	assert.Equal(0, check1.counter)
	assert.False(check1.done)
	assert.Equal(0, check2.counter)
	assert.False(check2.done)
	assert.Equal(0, check3.counter)
	assert.False(check3.done)

	complete = runner.Run(ctx)
	assert.Equal(1, check1.counter)
	assert.True(check1.done)
	assert.Equal(2, check2.counter)
	assert.True(check2.done)
	assert.Equal(3, check3.counter)
	assert.True(check3.done)
	assert.True(complete)

	check2 = &mockValidator{Turns: 2}
	check3 = &mockValidator{Turns: 3}
	runner.AddCheck(check2)
	runner.AddCheck(check3)

	complete = runner.Run(ctx)
	assert.Equal(1, check2.counter)
	assert.Equal(1, check3.counter)
	assert.False(check2.done)
	assert.False(check3.done)
	assert.False(complete)

	complete = runner.Run(ctx)
	assert.Equal(2, check2.counter)
	assert.Equal(3, check3.counter)
	assert.True(check2.done)
	assert.True(check3.done)
	assert.True(complete)
}

type mockValidator struct {
	Turns   int
	counter int
	done    bool
}

func (v *mockValidator) Name() string {
	return fmt.Sprintf("Mock %d", v.Turns)
}

func (v *mockValidator) Check(ctx *Context) (done bool, errs []error) {
	v.counter++
	if v.counter >= v.Turns {
		v.done = true
	}
	return v.done, []error{}
}
