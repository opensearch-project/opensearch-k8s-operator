#!/bin/bash
CLUSTER_NAME=opensearch-operator-tests

## Setup k3d cluster and prepare kubeconfig
k3d cluster create $CLUSTER_NAME --agents 2 --kubeconfig-switch-context=false --kubeconfig-update-default=false -p "30000-30005:30000-30005@agent:0"
k3d kubeconfig get $CLUSTER_NAME > kubeconfig
export KUBECONFIG=$(pwd)/kubeconfig

## Build controller docker image
cd ..
make docker-build

## Import controller docker image
k3d image import -c $CLUSTER_NAME controller:latest

## Install helm chart
helm install opensearch-operator ../charts/opensearch-operator --set manager.image.repository=controller --set manager.image.tag=latest --set manager.image.pullPolicy=IfNotPresent --namespace default --wait
cd functionaltests

## Run tests
go test -timeout 30m

## Delete k3d cluster
k3d cluster delete $CLUSTER_NAME
