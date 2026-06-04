The purpose of these directories is to have ALL HelmRelease objects in the
k-apps repo, including the ones that are generated dynamically.

By dynamically created, we mean HelmReleases which are created/maintained by
controllers as opposed to simply creating Appdeployments and updating git
repository.

Thanks to this approach, fetching charts and creating charts bundle can be done
without any extra information - simply by scanning k-apps repository. The
controllers that create these helmreleases create kustomizations instead that
change these charts accordingly.
