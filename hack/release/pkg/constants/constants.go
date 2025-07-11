package constants

const (
	KommanderAppPath       = "./applications/kommander/"
	KommanderAppMgmtPath   = "./applications/kommander-appmanagement/"
	CAPIMateDefaultVersion = "v0.0.0-dev.0"
	// SemverRegexp validates any semver (taken verbatim from semver specs).
	SemverRegexp = `v?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?` //nolint:lll // it's not readable anyway
)
