# static: repo analysis

This CLI tool uses [dkp-bloodhound](https://github.com/mesosphere/dkp-bloodhound) to traverse this repo and report metadata about services.

## Usage

```
Available Commands:
  bumps            Output all kommander-applications upstream chart bumps
  versions         Output all kommander-applications versions
  inspect          Output entire dkp-bloodhound node hierarchy for manual analysis

Flags:
  -t, --tag string           The tag to analyze
  -b, --branch string        The branch to analyze if 'tag' is not set (default "main")
  -o, --output-file string   Where to write output (default "/dev/stdout")
```
