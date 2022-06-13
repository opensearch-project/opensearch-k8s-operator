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
3. Create a new tag "helm-X.Y.Z" (edited) 