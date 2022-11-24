//go:build mage

package main

import (
	// mage:import asdf
	_ "github.com/mesosphere/daggers/mage/asdf"

	// mage:import lint
	_ "github.com/mesosphere/daggers/mage/precommit"
)
