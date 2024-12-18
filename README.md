[![Coverage Status](https://coveralls.io/repos/github/mesosphere/kommander-applications/badge.svg?branch=main)](https://coveralls.io/github/mesosphere/kommander-applications?branch=main)

# kommander-applications

This repo is dedicated to storing the HelmRelease and other info needed for Kommander's Applications.

### Pre Commit

This repo uses https://pre-commit.com/ to run pre-commit hooks. Please install pre-commit and run `pre-commit install` in the root directory before committing.

### Running Tests

You can run tests with `make go-test`. If your tests do not meet a certain coverage threshold, your build will fail.

### App Image Licenses

This repo contains a list of images, `licenses.d2iq.yaml` used to keep licenses up-to-date. This list is comprised of two sections: `ignore` and `resources`. `ignore` is a list of images that should be ignored when validating the license mappings.

Due to the automation of image version bumping, the original comments were unable to be retained and are listed below by corresponding image:

| Image                                                 | Description                                                                                                                                                                                                                                                                                                             |
|-------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `docker.io/mesosphere/kommander2-kubetools`           | The image is set of tools that is not built from source code. See: https://github.com/mesosphere/kommander (dir: docker)                                                                                                                                                                                                |
| `docker.io/nginxinc/nginx-unprivileged:1.22.0-alpine` | Fossa cannot scan nginx source code (C/C++) Original mapping: <pre>- container_image: docker.io/nginxinc/nginx-unprivileged:1.22.0-alpine<br>  sources:<br>    - url: https://github.com/nginxinc/docker-nginx-unprivileged<br>      ref: 82a186f7a71ca66269dba0a3eef1fb16f9121946<br>      license_path: LICENSE</pre> |
| `docker.io/bitnami/external-dns:0.13.4-debian-11-r2`  | List of bitnami containers that were mapped to build repository source code, but not to the actual bundled software source code                                                                                                                                                                                         |
| `docker.io/mesosphere/capimate:${kommander}`          | Note that this image is within `resources` rather than `ignore`. The `capimate` source code is in `capimate` subdirectory but it shares go.mod with main konvoy2 source code. `directory: capimate`                                                                                                                     |
| `gcr.io/kubecost1/frontend`                           | Partnership                                                                                                                                                                                                                                                                                                             |
| `gcr.io/kubecost1/cost-model`                         | Partnership                                                                                                                                                                                                                                                                                                             |

