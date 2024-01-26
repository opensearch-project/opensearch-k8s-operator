- [Contributing to the OpenSearch K8S Operator](#contributing-to-the-opensearch-k8s-operator)
  - [First Things First](#first-things-first)
  - [Ways to Contribute](#ways-to-contribute)
    - [Feature Requests](#feature-requests)
    - [Reporting bugs](#reporting-bugs)
    - [Contributing Code](#contributing-code)
  - [Developer Certificate of Origin](#developer-certificate-of-origin)
  - [Review Process](#review-process)
  - [Writing tests](#writing-tests)
    - [Functional tests](#functional-tests)

# Contributing to the OpenSearch K8S Operator

The OpenSearch K8S Operator is a community project that is built and maintained by people just like **you**. We're glad you're interested in helping out. There are several different ways you can do it, but before we talk about that, let's talk about how to get started.

## First Things First

1. **When in doubt, open an issue** - For almost any type of contribution, the first step is opening an issue. Even if you think you already know what the solution is, writing down a description of the problem you're trying to solve will help everyone get context when they review your pull request. If it's truly a trivial change (e.g. spelling error), you can skip this step -- but as the subject says, when in doubt, [open an issue](https://github.com/opensearch-project/opensearch-k8s-operator/issues).

2. **Only submit your own work**  (or work you have sufficient rights to submit) - Please make sure that any code or documentation you submit is your work or you have the rights to submit. We respect the intellectual property rights of others, and as part of contributing, we'll ask you to sign your contribution with a "Developer Certificate of Origin" (DCO) that states you have the rights to submit this work and you understand we'll use your contribution. There's more information about this topic in the [DCO section](#developer-certificate-of-origin).

## Ways to Contribute

There are several ways you can contribute to this project:

### Feature Requests

If you've thought of a way that the OpenSearch K8S Operator could be better, we want to hear about it. We track feature requests using GitHub, so please feel free to open an issue which describes the feature you would like to see, why you need it, and how it should work.

### Reporting bugs

If you have the time and infrastructure, please test the operator in different environments and with different usecases. Should you find a bug, please report it using the GitHub issue tracker. Please provide the following information in your report:

- Your environment (cloud provider, k8s distribution, etc.)
- A description what you did, what you expected to happen and what actually happened
- Your cluster spec YAML (if possible reduced to a minimal testcase) to allow others to reproduce your problem
- Any relevant kubectl outputs
- Operator logs if they have relevant information
- Opensearch and dashboards logs if they relevant information

### Contributing Code

As with other types of contributions, the first step is to [**open an issue on GitHub**](https://github.com/opensearch-project/OpenSearch/issues/new/choose). Opening an issue before you make changes makes sure that someone else isn't already working on that particular problem. It also lets us all work together to find the right approach before you spend a bunch of time on a PR. So again, when in doubt, open an issue.

Please see the [developer docs](./docs/developing.md) for details.

## Developer Certificate of Origin

The OpenSearch K8S Operator is an open source product released under the Apache 2.0 license (see either [the Apache site](https://www.apache.org/licenses/LICENSE-2.0) or the [LICENSE.txt file](./LICENSE.txt)). The Apache 2.0 license allows you to freely use, modify, distribute, and sell your own products that include Apache 2.0 licensed software.

We respect intellectual property rights of others and we want to make sure all incoming contributions are correctly attributed and licensed. A Developer Certificate of Origin (DCO) is a lightweight mechanism to do that.

The DCO is a declaration attached to every contribution made by every developer. In the commit message of the contribution, the developer simply adds a `Signed-off-by` statement and thereby agrees to the DCO, which you can find below or at [DeveloperCertificate.org](http://developercertificate.org/).

```text
Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the
    best of my knowledge, is covered under an appropriate open
    source license and I have the right under that license to
    submit that work with modifications, whether created in whole
    or in part by me, under the same open source license (unless
    I am permitted to submit under a different license), as
    Indicated in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including
    all personal information I submit with it, including my
    sign-off) is maintained indefinitely and may be redistributed
    consistent with this project or the open source license(s)
    involved.
```

We require that every contribution to OpenSearch is signed with a Developer Certificate of Origin. Additionally, please use your real name. We do not accept anonymous contributors nor those utilizing pseudonyms.

Each commit must include a DCO which looks like this

```text
Signed-off-by: Jane Smith <jane.smith@email.com>
```

You may type this line on your own when writing your commit messages. However, if your user.name and user.email are set in your git configs, you can use `-s` or `--signoff` to add the `Signed-off-by` line to the end of the commit message.

## Review Process

We deeply appreciate everyone who takes the time to make a contribution. We will review all contributions as quickly as possible. As a reminder, [opening an issue](https://github.com/Opster/opensearch-k8s-operator) discussing your change before you make it is the best way to smooth the PR process. This will prevent a rejection because someone else is already working on the problem, or because the solution is incompatible with the architectural direction.

During the PR process, expect that there will be some back-and-forth. Please try to respond to comments in a timely fashion, and if you don't wish to continue with the PR, let us know. If a PR takes too many iterations for its complexity or size, we may reject it. Additionally, if you stop responding we may close the PR as abandoned. In either case, if you feel this was done in error, please add a comment on the PR.

If we accept the PR, a [maintainer](MAINTAINERS.md) will merge your change and usually take care of backporting it to appropriate branches ourselves.

If we reject the PR, we will close the pull request with a comment explaining why. This decision isn't always final: if you feel we have misunderstood your intended change or otherwise think that we should reconsider then please continue the conversation with a comment on the PR and we'll do our best to address any further points you raise.

## Writing tests

Testing our code is an essential part of development. It ensures the features we develop are working as intended and that we did not break an existing feature or logic with our changes (also called regression tests). This project uses the following types of tests:

- Unit tests: These test a single function in isolation. They are implemented as normal go tests.
- Integration tests: These test an entire component of the operator and its interaction with kubernetes. We use [envtest](https://book.kubebuilder.io/reference/envtest.html) to provide a minimal kubernetes api-server and verify the component under test creates/updates kubernetes objects as expected.
- Functional tests: These test the operator as a whole and are used to verify interaction with a real kubernetes and opensearch cluster.

As a base rule: Every change you make should be covered by a unit or integration test. Choose an integration test if your function/component interacts with kubernetes, otherwise a unit tests with optional mocks is the way to go. If the change you make does not really have logic, you should still implement a simple test to act as a regression test to make sure this feature is not later on inadvertently crippeled or that a bug is not reintroduced.

### Functional tests

Functional tests look at the operator as a whole. They are intended to test functionality that requires a complete kubernetes cluster (and not just an api-server)  Whereas the unit and integration tests are part of the operator code (as `*_test.go` files), the functional tests are implemented as a separate go module (in the `opensearch-operator/functionaltests` directory) but still use the normal go testing mechanisms (meaning they can be run with `go test`).

The flow of the functional tests is as follows:

1. Setup a k3d cluster
2. Build the docker image for the current operator code
3. Deploy the operator using helm
4. Run `go test`. Each test follows the same structure:
   1. Deploy an `OpenSearchCluster` object
   2. Wait for and verify conditions in kubernetes and/or opensearch
   3. Tear down the opensearch cluster

The flow is implemented as a Github Actions Workflow that is run for each pull request automatically. In addition you can run them locally by using the script `execute_tests.sh` in the directory `opensearch-operator/functionaltests`. You will need k3d and helm installed on your system.

For each functional test two files are needed: A go file with the test code (`<testname>_test.go`) and a yaml file with the needed kubernetes objects (`<testname>.yaml`).

The functionaltests module has some helper functions:

- `CreateKubernetesObjects(name string)`: Reads in a yaml file `<name>.yaml` and creates the kubernetes objects defined in it
- `Cleanup(name string)`: Reads in a yaml file `<name>.yaml` and deletes the kubernetes objects defined in it
- `ExposePodViaNodePort(selector map[string]string, namespace string, nodePort, targetPort int32)`: Creates a NodePort service to expose pods outside the k3d cluster (use a port in range `30000-30005`)
- `CleanUpNodePort(namespace string, nodePort int32)`: Deletes a NodePort service

When adding new tests plase try to follow the same structure as existing tests. Also keep in mind that functional tests take a lot longer to run than a unit test, as such only add them if needed.
