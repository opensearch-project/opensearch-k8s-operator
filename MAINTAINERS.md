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

Under the releases process we can find two main actions
#### Operator release
Responsible to build and publish new Opensearch-k8s-operator images, builds from the 'release.yaml' workflow that triggers from new tag creation with 'v' prefix.
#### Helm release
Responsible for Helm chart repo update, new release and update the index in the `gh-pages` branch, will also publish version to Artifacthub. 

### Steps
So for a release we need the following manual steps:
1. Create a new tag "vXY..Z"
2. Wait until pipeline is finished
3. in case of CRDs and manifest change
   1. run 'make manifest' && 'make generate'
   2. run kustomize build > output.yaml
   3. separate the yamls and add each resource as a yaml file to charts/opensearch-operator/templates folder (that phase will improve to be part of release process)
4. Edit 'version' under charts/opensearch-operator/Chart.yaml (edit also 'appVersion' in case that a new applicative version has released )
   1. after editing a new release will upload to artifactHub
