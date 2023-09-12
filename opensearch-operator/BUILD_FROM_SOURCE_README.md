# Build Instructions

The base tag, this release is branched from is `v2.4.0`


Create Environment Variables

```
export DOCKER_REPO=<Docker Repository>
export DOCKER_NAMESPACE=<Docker Namespace>
export DOCKER_TAG=v2.4.0
```

Build and Push Images

```
# Build and push opensearch-k8s-operator

docker build -t ${DOCKER_REPO}/${DOCKER_NAMESPACE}/opensearch-k8s-operator:${DOCKER_TAG} .
docker push ${DOCKER_REPO}/${DOCKER_NAMESPACE}/opensearch-k8s-operator:${DOCKER_TAG}
```
