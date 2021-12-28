In case when the [PR](https://github.com/chartmuseum/charts/pull/32) with our
patches does not get merged, upgrading the helm chart tarball is as follows:

This the vanila helm chart, pulled from:

https://github.com:chartmuseum/charts

patched with:

https://github.com/chartmuseum/charts/pull/32

and packaged with:

helm package chartmuseum
