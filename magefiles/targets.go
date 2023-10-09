//go:build mage

package main

import (
	// mage:import asdf
	_ "github.com/mesosphere/d2iq-daggers/catalog/asdf"

	// mage:import lint
	_ "github.com/mesosphere/d2iq-daggers/catalog/precommit"
)
