# Application Specific Tests

These tests are intended to test the configuration of each app. They are not intended to test the functionality 
of the application.The tests are written in Ginkgo and Gomega.

Most of the tests run in kind locally. The notable exception are the Rook Ceph tests and the Velero Local backup tests 
which don't work on ARM (Apple Silicon) running Colima. There are some images that we are not building for ARM right now.

## Prerequisites

- [Ginkgo](https://onsi.github.io/ginkgo/)
- [Kind](https://kind.sigs.k8s.io/)
- [Docker](https://www.docker.com/)
- [Colima]()

Setup the environment:

If you are using `colima`, you can set the `DOCKER_HOST` environment variable to the socket used by `colima`. 
This will allow the docker api to use the `colima` docker instance. Rancher Desktop will also need this.

```bash
export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
```

## Running the tests

To run the tests, execute the following command:

```bash
# For all tests
cd apptests
ginkgo appscenarios

# For an individual install test
ginkgo --label-filter="kommander-flux && install" appscenarios

# Or for an upgrade test
ginkgo --label-filter="kommander-flux && upgrade" appscenarios
```

## Test Cases

| Test Case   | Test Label   | Description                               |
|-------------|--------------|-------------------------------------------|
| CertManager | cert-manager | Test the CertManager configuration        |
| Karma       | karma        | Test the Karma configuration              |
| KubeCost    | kubecost     | Test the KubeCost configuration           |
| Reloader    | reloader     | Test the Reloader configuration           |
| Traefik     | traefik      | Test the Traefik configuration            |
| Karma       | karma        | Test the Karma and Traefik configuration  |
| Flux        | flux         | Test the Flux configuration               |

