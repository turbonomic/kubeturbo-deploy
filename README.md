# kubeturbo-deploy

[![Made with Operator SDK](https://img.shields.io/badge/Made%20with-Operator%20SDK%20-EE0000?logo=data:image/svg%2bxml;base64,PHN2ZyBjbGFzcz0iYmkgYmktbGlnaHRuaW5nLWNoYXJnZS1maWxsIiBmaWxsPSIjRkZGIiBoZWlnaHQ9IjE2IiB2aWV3Qm94PSIwIDAgMTYgMTYiIHdpZHRoPSIxNiIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cGF0aCBkPSJNMTEuMjUxLjA2OGEuNS41IDAgMCAxIC4yMjcuNThMOS42NzcgNi41SDEzYS41LjUgMCAwIDEgLjM2NC44NDNsLTggOC41YS41LjUgMCAwIDEtLjg0Mi0uNDlMNi4zMjMgOS41SDNhLjUuNSAwIDAgMS0uMzY0LS44NDNsOC04LjVhLjUuNSAwIDAgMSAuNjE1LS4wOXoiLz48L3N2Zz4=)](https://sdk.operatorframework.io/) [![Operator SDK Version](https://img.shields.io/badge/Operator%20SDK%20version-1.34.1%20-EE0000)](https://github.com/operator-framework/operator-sdk/releases/tag/v1.34.1) [![Go Version](https://img.shields.io/badge/Go%20version-1.21.8%20-00ADD8)](https://go.dev/)

## Description

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) and this operator is used to install [Kubeturbo](https://github.com/turbonomic/kubeturbo/tree/master/deploy/kubeturbo).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster


## Getting Started

You’ll need a Kubernetes cluster to run against. It is recommended that you run against a remote cluster.

**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

If you are using a Turbonomic VM, it is also recommended that you use your VM's docker environment when building the Operator docker image.

### Running on the cluster

1. Build the Operator image:

```sh
# Build local docker image
make docker-build

# Build multi arch image and push to registry
make docker-buildx
```

2. Deploy the Operator to the cluster. You can set the docker image version by setting the following environment variable before running the commands.

If `VERSION` is not set, the default value from the Makefile will be used. See the Makefile for other variables that can be set, such as `OPERATOR_NAME`, `REGISTRY` and `NAMESPACE`.:

```sh
# Deploy operator with the latest push tag
make deploy

# example of deploying the operator in turbo ns with a specific version
NAMESPACE=turbo VERSION=tmp-8.12.5-SNAPSHOT make deploy
```

**Note:** This assumes that the Operator docker image is accessible to the cluster

3. Install a Custom Resource instance:

Since the customize resources definition might not be available at the cluster you 
work with, it's not a bad idea to always install the CRDs before applying your CR:

```sh
make install
```

Sample Kubeturbo CR can be found in `config/samples/`. You can also create your own Kubeturbo CR of your choice. Install the instance using `kubectl apply`, e.g.:

```sh
kubectl apply -f <Kubeturbo CR YAML filepath>
```

#### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall
```

#### Undeploy Operator

UnDeploy the operator to the cluster:

```sh
make undeploy

# Similar to make deploy, if you specific to install the operator 
# in a specific namespace please export that namespace as well
NAMESPACE=turbo make undeploy
```

### Running the go based operator on local machine

1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

You can also use your IDE to run the operator, here is a sample for VSCode for debug purposes

```json
{
    // Use IntelliSense to learn about possible attributes.
    // Hover to view descriptions of existing attributes.
    // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Kubeturbo operator",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd",
            "args": [],
            "env": {
                "WATCH_NAMESPACE": "turbonomic"
            },
        }
    ]
}
```

#### Testing e2e

The end to end testing is to test if the Kubeturbo operator can be deploy successfully and to verify if the Kubeturbo can be applied as well. The test utilizes to use `Kind` cluster a the host cluster and then :
1. do `make install deploy` to install the operator
2. check if the operator is running
3. [deploy the sample Kubeturbo CR](https://github.ibm.com/turbonomic/kubeturbo-deploy/blob/staging/config/samples/charts_v1_kubeturbo.yaml) in the same namespace where the operator locates
4. <TODO: check generated resources for the Kubeturbo CR>

You will need to start a kind cluster before running the e2e test
```bash
# install required tools
make kind kubectl

# 
make describe-vars
# REGISTRY:        icr.io/cpopen
# OPERATOR_NAME:   kubeturbo-operator
# VERSION:         tmp-8.12.5-SNAPSHOT
# NAMESPACE:       turbonomic
# KUBECTL:         kubectl
# KIND:            /root/repo/kubeturbo-deploy/bin/kind
# KIND_CLUSTER:    kubeturbo-operator-kind
# KIND_KUBECONFIG: /root/.kube/kind-config

# KIND_CLUSTER is optional, and it's default to kind
KIND_CLUSTER=<NAME> make create-kind-cluster
```

To run the e2e test, you can simply do `make e2e-test`

To run the e2e test in a debug mode you can use the following config in your VSCode
```json
{
    // Use IntelliSense to learn about possible attributes.
    // Hover to view descriptions of existing attributes.
    // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug e2e test",
            "type": "go",
            "request": "launch",
            "mode": "test",
            "program": "${workspaceFolder}/test/e2e",
            "args": [
                "-ginkgo.v"
            ],
            "env": {
                "KUBECTL": "<KUBECTL from make>",
                "KIND": "<KIND from make>",
                "KIND_CLUSTER": "<KIND_CLUSTER from make>",
                "KIND_KUBECONFIG": "<KIND_KUBECONFIG from make>"
            },
        },
    ]
}
```


### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

### Educational Resources

- https://sdk.operatorframework.io/docs/building-operators/golang/
- https://book.kubebuilder.io/
- https://book.kubebuilder.io/reference/markers/crd.html
- https://book.kubebuilder.io/reference/markers/rbac.html
- https://kubectl.docs.kubernetes.io/references/kustomize/
- https://kubernetes.io/docs/reference/access-authn-authz/rbac/
- https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
- https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md
