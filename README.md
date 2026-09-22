# nkp-catalog-tests

**nkp-catalog-tests** is a standalone, reusable application testing framework for the Nutanix Kubernetes Platform (NKP).

## Usage

Catalogs consume this repository as a single Go module. HelmRelease install/upgrade suites import `github.com/nutanix-cloud-native/nkp-catalog-tests/catalog` (`InitSuite`, `NewAppScenario`, `SetupKindCluster`, `WaitForFluxCRDs`).

While this repository is private, `proxy.golang.org` cannot fetch it. Set:

```bash
export GOPRIVATE=github.com/nutanix-cloud-native/nkp-catalog-tests
```

Add it to `go.mod`:

```
module github.com/nutanix-cloud-native/nkp-ai-applications-catalog

go 1.22

require (
    github.com/nutanix-cloud-native/nkp-catalog-tests v0.1.0
    github.com/onsi/ginkgo/v2 v2.19.0
    github.com/onsi/gomega v1.33.1
)
```

Or:

```bash
go get github.com/nutanix-cloud-native/nkp-catalog-tests@v0.1.0
```

To confirm `proxy.golang.org` indexed the tag:

```bash
curl -fsSL "https://proxy.golang.org/github.com/nutanix-cloud-native/nkp-catalog-tests/@v/v0.1.0.info"
GOPROXY=https://proxy.golang.org,direct go list -m github.com/nutanix-cloud-native/nkp-catalog-tests@v0.1.0
```

### `NKP_CATALOG_DIR`

This module does not ship catalog application manifests. Application tests and environment helpers read them from an external catalog checkout (for example `nkp-ai-applications-catalog` or `kommander-applications`).

Set `NKP_CATALOG_DIR` to that checkout's root — the directory that contains `applications/`:

```bash
export NKP_CATALOG_DIR=/path/to/nkp-catalog
```

The library then resolves application manifests under `$NKP_CATALOG_DIR/applications/<app>/<version>/`.

The value may be an absolute path, or a path relative to the process working directory. Prefer an absolute path when running `ginkgo` or `go test` from a package subdirectory (`ginkgo` sets the working directory to the test package).

If `NKP_CATALOG_DIR` is unset, the library falls back to this repository's root. That tree has no `applications/` or `common/` directories, so Ginkgo application tests fail unless the variable is set.

`NKP_CATALOG_DIR` does not affect fixtures in this repository's `testdata/` directory.

## Local Development

This repository uses [Devbox](https://www.jetpack.io/devbox) to ensure reproducible and consistent toolchains.

1. Install Devbox if you haven't already.
2. Run `devbox shell` to enter the environment, which provisions Go 1.22 and other required tools.
3. Export `NKP_CATALOG_DIR` to a catalog checkout before running Ginkgo application tests (see above). See [tests-doc.md](tests-doc.md) for test labels and run commands.

## Contributing

1. Ensure you have installed and enabled the pre-commit hooks (`pre-commit install`) to run `golangci-lint` and `shfmt` automatically.
2. Push your branch and open a Pull Request.
3. Once approved, merge to the `main` branch.

## Releasing

Versions are calculated by [release-please](https://github.com/googleapis/release-please). Do not push version tags by hand.

PR titles must be conventional commits (enforced in CI). [conventional-release-labels](https://github.com/bcoe/conventional-release-labels) then applies the matching GitHub label. release-please maps those to the next tag:

| PR title | Label | Bump |
| --- | --- | --- |
| `feat:` | `feature` | minor |
| `fix:` | `fix` | patch |
| `feat!:` or `BREAKING CHANGE:` | `breaking` | major |

GitHub Actions in the `nutanix-cloud-native` org cannot create the initial release PR. Create it locally with write access, then [release-please-action](https://github.com/googleapis/release-please-action) keeps it up to date.

### Create the release PR

Checkout `main` locally, then:

```bash
devbox run release-please release-pr \
  --repo-url nutanix-cloud-native/nkp-catalog-tests \
  --token "$(gh auth token)"
```

To force a version:

```bash
devbox run release-please release-pr \
  --repo-url nutanix-cloud-native/nkp-catalog-tests \
  --token "$(gh auth token)" \
  --release-as 0.1.0
```

### Cut the release

Checkout the release PR, sign the commit, and force-push:

```bash
gh pr checkout <RELEASE_PR_NUMBER>
git commit --gpg-sign --amend --no-edit
git push --force-with-lease
```

Merging the PR creates `vX.Y.Z`, the GitHub Release, and requests `proxy.golang.org` to index `github.com/nutanix-cloud-native/nkp-catalog-tests@<tag>`.

To re-index an existing tag, run Actions → Release → Run workflow and pass the tag.
