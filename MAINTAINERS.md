# Maintainers

| Maintainer | GitHub ID | Affiliation |
| --------------- | --------- | ----------- |
| Idan Levy | [idanl21](https://github.com/idanl21) | Opster |
| Ido | [ido-opster](https://github.com/ido-opster) | Opster |
| Dan Bason | [dbason](https://github.com/dbason) | SUSE |
| Sebastian Woehrl | [swoehrl-mw](https://github.com/swoehrl-mw) | MaibornWolff |
| Prudhvi Godithi | [prudhvigodithi](https://github.com/prudhvigodithi) | Amazon |

The following sections explain what maintainers do in this repo, and how they should be doing it. If you're interested in contributing, see [CONTRIBUTING](CONTRIBUTING.md).

## Release process

To release a new version of the operator open Github in the browser, navigate to "Actions", select the workflow `Prepare and publish release`, select "Run workflow", then enter the version of the release (semver, x.y.z) and click "Run workflow". After a few seconds a new workflow run will start. It will do the following:

* Run the test suite to make sure the version is functional
* Update the helm chart for operator with the newest CRD YAMLs
* Update `version` and `appVersion` in both the charts
* Commit and push the chart changes
* Tag the commit
* Build and push the docker image
* Create a new helm chart release using github pages
* Create a new release on github in draft mode

After the workflow has completed, navigate to releases and edit the new release. Generate a changelog, add any other needed information (upgrade instructions, warnings, incompatibilities, etc.) and then publish the release.

In case it is needed you can also manually tag a commit for release, this will trigger a workflow that stars with the "Build and push the docker image" step. Make sure CRDs are up-to-date in the helm chart.

After releasing a new version add it to the compatiblity matrix in the [README](README.md).
