//go:build mage

package main

import (
	"context"

	"github.com/magefile/mage/mg"

	"github.com/mesosphere/daggers/mage/help"

	// Target imports

	// mage:import lint
	"github.com/mesosphere/daggers/mage/precommit"
)

// Lint runs all the linting tasks.
func Lint(ctx context.Context) {
	mg.CtxDeps(ctx, precommit.Precommit)
}

// Help targets.
// Following targets are just to simple wrap the help targets to select which help targets to enable in mage.

// Help is a name prefix for help targets. This is required for not collusion with other targets
type Help mg.Namespace

func (Help) Precommit() {
	mg.Deps(help.Precommit)
}
