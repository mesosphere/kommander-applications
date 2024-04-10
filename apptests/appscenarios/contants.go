package appscenarios

import "time"

const (
	// General test parameters
	pollInterval           = 2 * time.Second
	kommanderNamespace     = "kommander"
	kommanderFluxNamespace = "kommander-flux"

	// Default path to upgrade k-apps repository
	defaultUpgradeKAppsRepoPath = ".work/upgrade/kommander-applications"
)
