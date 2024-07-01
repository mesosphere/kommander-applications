package appscenarios

import "time"

const (
	// General test parameters
	pollInterval           = 2 * time.Second
	kommanderNamespace     = "kommander"
	kommanderFluxNamespace = "kommander-flux"

	// Environment variables
	upgradeKappsRepoPathEnv = "UPGRADE_KAPPS_REPO_PATH"

	// Default path to upgrade k-apps repository
	defaultUpgradeKAppsRepoPath = ".work/upgrade/kommander-applications"

	// priority class names
	dkpHighPriority               = "dkp-high-priority"
	systemClusterCriticalPriority = "system-cluster-critical"
	dkpCriticalPriority           = "dkp-critical-priority"

	// Velero constants
	kubetoolsImageRepository = "bitnami/kubectl"
	kubetoolsImageTag        = "1.29.6"
)
