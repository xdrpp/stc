// XXX consider using golang.org/x/sync/errgroup instead.

package stcdetail

import (
	"fmt"
	"strings"
)

// An error type that groups together a bunch of errors and renders
// them separated by newlines.
type Errors []error

func (errs Errors) Error() string {
	out := strings.Builder{}
	for i := range errs {
		fmt.Fprintln(&out, errs[i].Error())
	}
	return strings.TrimRight(out.String(), "\r\n")
}

// Abstratction to run a bunch of functions concurrently and wait for
// them all to finish.
type Async struct {
	jobs []chan error
}

func (a *Async) Run(fn func() error) {
	c := make(chan error, 1)
	a.jobs = append(a.jobs, c)
	go func() {
		c <- fn()
	}()
}

func (a *Async) RunVoid(fn func()) {
	c := make(chan error, 1)
	a.jobs = append(a.jobs, c)
	go func() {
		fn()
		c <- nil
	}()
}

// Wait for all jobs that have been run and return their errors (if
// any).  Leaves the Async in a pristine state that can be reused.
func (a *Async) Wait() error {
	jobs := a.jobs
	a.jobs = nil
	var errs Errors
	for i := range jobs {
		e := <-jobs[i]
		if e != nil {
			errs = append(errs, e)
		}
	}
	if errs != nil {
		return errs
	}
	return nil
}
