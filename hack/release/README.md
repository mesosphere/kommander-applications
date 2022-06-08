# kommander-applications Release CLI helper tool

This CLI tool is leveraged by `gh-dkp` to manage the release of `kommander-applications`. 

There are a few tasks that need to happen in this repository both pre and post release.

## Pre-release

Before releasing a new `kommander-applications` version, the kommander and kommander-appmanagement `HelmReleases` 
need to be updated with the new Kommander chart version. 

This tool is intended to be run using the git ref of each release branch to ensure we are using the correct version of the tool for each release.

```bash
go run github.com/mesosphere/kommander-applications/hack/release@<git ref> pre-release --chart-version <chart version> --kommander-applications-repo </path/to/repo>
```

This command will result in:
 * The chart version being updated in the Kommander `HelmRelease` files in the local copy of the repo
