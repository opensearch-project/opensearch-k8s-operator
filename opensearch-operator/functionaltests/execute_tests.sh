#!/bin/bash
CLUSTER_NAME=opensearch-operator-tests

## Setup k3d cluster and prepare kubeconfig
k3d cluster create $CLUSTER_NAME \
    --servers 1 --agents 2 \
    -p "30000-30005:30000-30005@agent:0" \
    --k3s-arg "--kubelet-arg=eviction-hard=nodefs.available<1Mi@all" \
    --kubeconfig-switch-context=false \
    --kubeconfig-update-default=false \
    --image=rancher/k3s:v1.34.1-k3s1
k3d kubeconfig get $CLUSTER_NAME > kubeconfig
export KUBECONFIG=$(pwd)/kubeconfig

## Build controller docker image
cd ..
make docker-build

## Import controller docker image
k3d image import -c $CLUSTER_NAME controller:latest

## Install helm chart
helm install opensearch-operator ../charts/opensearch-operator \
          -f functionaltests/helmtests/ci-operator-values.yml \
          --namespace default --wait

helm install opensearch-cluster ../charts/opensearch-cluster \
            -f functionaltests/helmtests/ci-cluster-values.yml \
            --wait

cd functionaltests

## Run tests
go test ./operatortests -timeout 45m -ginkgo.v
go test ./helmtests -timeout 20m
## Delete k3d cluster
k3d cluster delete $CLUSTER_NAME
