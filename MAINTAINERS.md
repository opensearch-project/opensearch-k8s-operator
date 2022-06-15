## Maintainers

| Maintainer | GitHub ID | Affiliation |
| --------------- | --------- | ----------- |
| Idan Levy | [idanl21](https://github.com/idanl21) | Opster |
| Ido | [ido-opster](https://github.com/ido-opster) | Opster |
| Dan Bason | [dbason](https://github.com/dbason) | SUSE |
| Sebastian Woehrl | [swoehrl-mw](https://github.com/swoehrl-mw) | MaibornWolff |
| Prudhvi Godithi | [prudhvigodithi](https://github.com/prudhvigodithi) | Amazon |

[This document](https://github.com/Opster/opensearch-k8s-operator/.github/blob/main/MAINTAINERS.md) explains what maintainers do in this repo, and how they should be doing it. If you're interested in contributing, see [CONTRIBUTING](CONTRIBUTING.md).


# Opensearch Operator New version release guide

This guide will explain the new version release process in Opensearch-k8s-Operator

## Logic

Under the relases process we can find two main actions
* ####Operator release
  Responsible to build and publish new Opensearch-k8s-operator images, builds from the 'release.yaml' workflow that triggers from new tag creation with 'v' prefix.
* ####Helm release
  Respoinsable on Helm chart repo update, will set helm chart version to version from tag and will Run action to build chart and publish on Artifacthub. builds from the 'helm-release.yaml' workflow that triggers from new tag creation with 'helm' prefix.

###Steps
So for a release we need the following manual steps:
1. Create a new tag "vX.Y.Z"
2. Wait until pipeline is finished
3. Create a new tag "helm-X.Y.Z" (edited) ד