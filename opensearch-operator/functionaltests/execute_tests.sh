#!/bin/bash
CLUSTER_NAME=opensearch-operator-tests

## Setup k3d cluster and prepare kubeconfig
k3d cluster create $CLUSTER_NAME --agents 2 --kubeconfig-switch-context=false --kubeconfig-update-default=false -p "30000-30005:30000-30005@agent:0" --image=rancher/k3s:v1.31.4-k3s1
k3d kubeconfig get $CLUSTER_NAME > kubeconfig
export KUBECONFIG=$(pwd)/kubeconfig

## Build sidecar docker image and import into k3d
cd ../../operator-sidecar
IMG=operator-sidecar:dev make docker-build
k3d image import -c $CLUSTER_NAME operator-sidecar:dev

## Build controller docker image and import into k3d
cd ../opensearch-operator
make docker-build
k3d image import -c $CLUSTER_NAME controller:latest

## Install helm chart
helm install opensearch-operator ../charts/opensearch-operator \
  -f functionaltests/helmtests/ci-operator-values.yml \
  --set operatorSidecar.image=operator-sidecar:test \
  --namespace default --wait

cd functionaltests

kubectl apply -f rbac.yaml

## Run tests
go test ./operatortests -timeout 30m
go test ./helmtests -timeout 20m

## Delete k3d cluster
k3d cluster delete $CLUSTER_NAME
