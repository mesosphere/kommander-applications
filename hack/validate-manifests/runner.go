package main

import (
	"sync"
)

const (
	termRed   = "\x1b[31m"
	termGreen = "\x1b[32m"
	termReset = "\x1b[0m"
)

type Validator interface {
	Name() string
	Check(ctx *Context) (done bool, errs []error)
}

type Runner struct {
	queue []Validator
	sync.Mutex
}

func (r *Runner) AddCheck(check Validator) {
	r.Lock()
	r.queue = append(r.queue, check)
	r.Unlock()
}

func (r *Runner) Run(ctx *Context) bool {
	success := true
	for {
		r.Lock()
		checkCount := len(r.queue)
		checks := r.queue
		r.queue = []Validator{}
		r.Unlock()

		wg := sync.WaitGroup{}
		wg.Add(checkCount)
		dones := make([]bool, checkCount)
		errs := make([][]error, checkCount)
		for i, check := range checks {
			go func(i int, check Validator) {
				dones[i], errs[i] = check.Check(ctx)
				wg.Done()
			}(i, check)
		}

		wg.Wait()

		anyDone := false
		reQueue := make([]Validator, 0, checkCount)
		for i, check := range checks {
			if dones[i] {
				anyDone = true
				errors := errs[i]
				if len(errors) == 0 {
					ctx.Infof(" %s✓%s %s", termGreen, termReset, check.Name())
				} else {
					success = false
					ctx.Infof(" %s✗%s %s", termRed, termReset, check.Name())
				}
				for _, err := range errors {
					ctx.Errorf(err, check.Name())
				}
			} else {
				reQueue = append(reQueue, check)
			}
		}

		r.Lock()
		anyAdded := len(r.queue) > 0
		r.queue = append(r.queue, reQueue...)
		r.Unlock()

		if !(anyDone || anyAdded) {
			break
		}
	}

	r.Lock()
	if len(r.queue) > 0 {
		success = false
		ctx.Errorf(nil, "didn't execute everything, missing dependencies?")
		for _, check := range r.queue {
			ctx.Infof("- %s", check.Name())
		}
	}
	r.Unlock()
	return success
}
