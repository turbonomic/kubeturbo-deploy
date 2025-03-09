# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
export VERSION ?= 8.15.4-SNAPSHOT
export DEFAULT_KUBETURBO_VERSION=$(shell echo $(VERSION) | sed -E 's/([1-9][0-9]*)\.([1-9][0-9]*)\.([0-9]+)00(.*)/\1.\2.\3\4/')

# build info
REMOTE_URL=$(shell git config --get remote.origin.url)
GIT_COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
REVISION=$(shell git show -s --format=%cd --date=format:'%Y%m%d%H%M%S000')
BUILD_TIMESTAMP=$(shell date +'%Y%m%d%H%M%S000')

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# *-bundle:$VERSION and *-catalog:$VERSION.
export REGISTRY ?= icr.io/cpopen
# temporarily adding "-new" surffix to help on transition, we might need to remove it in the end
export OPERATOR_NAME ?= kubeturbo-operator

# Use this value to set the Docker image registry
ifneq ($(origin REGISTRY), undefined)
_REGISTRY_PREFIX := $(REGISTRY)/
endif

IMAGE_TAG_BASE ?= $(_REGISTRY_PREFIX)$(OPERATOR_NAME)

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.34.1

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.3

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen kustomize ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=kubeturbo-operator crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(KUSTOMIZE) build config/crd -o config/crd/bases/charts.helm.k8s.io_kubeturbos.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: export_yaml
export_yaml: export_operator_yaml_bundle
	sh ./scripts/export_yamls.sh

export YAML_BUNDLE_DIR ?= deploy/kubeturbo_operator_yamls
.PHONY: export_operator_yaml_bundle
export_operator_yaml_bundle: manifests kustomize
	mkdir -p $(YAML_BUNDLE_DIR)
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | sed 's|__NAMESPACE__|$(NAMESPACE)|g' > $(YAML_BUNDLE_DIR)/operator-bundle.yaml

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
export TESTING_LOGGING_LEVEL ?= WARN
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e: create-kind-cluster kubectl
	go test ./test/e2e/ -v -ginkgo.v

GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.54.2
golangci-lint: $(LOCALBIN)
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: buildInfo
buildInfo:
		$(shell test -f git.properties && rm -rf git.properties)
		@echo 'turbo-version.remote.origin.url=$(REMOTE_URL)' >> git.properties
		@echo 'turbo-version.commit.id=$(GIT_COMMIT)' >> git.properties
		@echo 'turbo-version.branch=$(BRANCH)' >> git.properties
		@echo 'turbo-version.branch.version=$(VERSION)' >> git.properties
		@echo 'turbo-version.commit.time=$(REVISION)' >> git.properties
		@echo 'turbo-version.build.time=$(BUILD_TIMESTAMP)' >> git.properties

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: docker-precheck ## Build docker image with the manager.
	$(CONTAINER_TOOL) build --no-cache --build-arg VERSION=$(VERSION) --build-arg DEFAULT_KUBETURBO_VERSION=$(DEFAULT_KUBETURBO_VERSION) -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: docker-precheck ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name $(OPERATOR_NAME)-builder
	$(CONTAINER_TOOL) buildx use $(OPERATOR_NAME)-builder
	- $(CONTAINER_TOOL) buildx build --label "git-commit=$(GIT_COMMIT)" --label "git-version=$(VERSION)" --build-arg VERSION=$(VERSION) --build-arg DEFAULT_KUBETURBO_VERSION=$(DEFAULT_KUBETURBO_VERSION) --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm $(OPERATOR_NAME)-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize kubectl ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) $(KUBECONFIG_STR) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize kubectl ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) $(KUBECONFIG_STR) delete --ignore-not-found=$(ignore-not-found) -f -

# Developer Edit: added NAMESPACE variable and sed command
export NAMESPACE ?= turbo
.PHONY: deploy
deploy: manifests kustomize kubectl ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | sed 's|__NAMESPACE__|$(NAMESPACE)|g' | $(KUBECTL) $(KUBECONFIG_STR) apply -f -

.PHONY: undeploy
undeploy: kustomize kubectl ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | sed 's|__NAMESPACE__|$(NAMESPACE)|g' | $(KUBECTL) $(KUBECONFIG_STR) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location copy api folder and the internal folder for docker build
docker-precheck:
	mkdir -p api
	mkdir -p internal/controller/

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries, those exported vars are required in test while using make cil
export KUBECTL ?= $(shell command -v kubectl >/dev/null 2>&1 && echo kubectl || echo $(LOCALBIN)/kubectl)
export KIND ?= $(shell command -v kind >/dev/null 2>&1 && echo kind || echo $(LOCALBIN)/kind)
export KIND_CLUSTER ?= $(OPERATOR_NAME)-kind
export KIND_KUBECONFIG ?= $(HOME)/.kube/kind-config
export HELM ?= $(LOCALBIN)/helm
export KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest


## Tool Versions
KUSTOMIZE_VERSION ?= v5.2.1
CONTROLLER_TOOLS_VERSION ?= v0.15.0

.PHONY: kubectl
kubectl: $(LOCALBIN) ## Download kubectl locally if necessary.
	@if ! command -v kubectl >/dev/null 2>&1 ; then \
		test -s $(LOCALBIN)/kubectl || \
		curl -Lo $(LOCALBIN)/kubectl "https://storage.googleapis.com/kubernetes-release/release/$(shell curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$(shell go env GOOS)/$(shell go env GOARCH)/kubectl" && \
		chmod +x "$(LOCALBIN)/kubectl"; \
	fi

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: kind
kind: $(LOCALBIN) ## Download kind locally if necessary.
	@if ! command -v kind >/dev/null 2>&1 ; then \
		test -s $(LOCALBIN)/kind || GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@latest; \
	fi

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

YQ ?= $(LOCALBIN)/yq
YQ_TOOLS_VERSION ?= v4.30.4

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	test -s $(LOCALBIN)/yq || GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_TOOLS_VERSION)


PYTHON = $(LOCALBIN)/python3

python: $(PYTHON)  ## Install Python locally if necessary. Darwin OS is specific to mac users if running locally
$(PYTHON):
	@if ! command -v python3 >/dev/null 2>&1; then \
		mkdir -p $(LOCALBIN); \
		if [ `uname -s` = "Darwin" ]; then \
			brew install python@3; \
		else \
			sudo apt update && sudo apt install python3; \
		fi \
	fi
	# Ensure the bin directory exists before linking
	@mkdir -p $(LOCALBIN)
	ln -sf `command -v python3` $(PYTHON)



# This parameter adjusts the patch version of the operator release. It suffix the patch number with a zero(x.y.z - 'z' is the operator version patch number)
# This change in patch number incrementation strategy offers flexibility and room for post release fixes of operator bundle
OPERATOR_RELEASE_VERSION_PATCH := $(shell echo $(OPERATOR_RELEASE_VERSION) | sed -E 's/(^[0-9]+\.[0-9]+\.)([1-9])$$/\1\20/')
# This parameter adjusts the OLM inclusive range of the operator version
OPERATOR_OLM_INCLUSIVE_RANGE_VERSION ?= 8.7.5
# This parameter adjusts the OLM inclusive range to prefix with '-beta.1' of operator version if the release channel is beta
OPERATOR_OLM_INCLUSIVE_BETA_VERSION ?= beta.1
# This parameter is a place holder for 'beta' keyword
OPERATOR_BETA_RELEASE_FILTER ?= beta
# This parameter is a place holder for 'SNAPSHOT' keyword to pull the SNAPSHOT image version for beta release
OPERATOR_BETA_RELEASE_VERSION_SNAPSHOT ?= SNAPSHOT
OPERATOR_CERTIFIED ?= kubeturbo-certified
OPERATOR_BUNDLE_DIR ?= certified-operator-bundle
OPERATOR_BUNDLE_CONFIG_DIR ?= certified-bundle-config
# This is a path to copy the crd into operator bundle
OPERATOR_CRD_FILE_PATH ?= config/crd/bases/charts.helm.k8s.io_kubeturbos.yaml
# This is a path to copy cluster role permission into csv
CLUSTER_PERMISSION_ROLE_YAML_FILE_PATH ?= config/rbac/kubeturbo-operator-cluster-role.yaml
CERTIFIED_OPERATOR_CLUSTER_SERVICE_VERSION_YAML_FILE_PATH ?= $(OPERATOR_BUNDLE_DIR)/manifests/kubeturbo-certified.clusterserviceversion.yaml
# This is a path to github repo to verify the existing operator bundle versions released
GITHUB_REPO_URL := https://api.github.com/repos/turbonomic/certified-operators/contents/operators/kubeturbo-certified
.PHONY: build-certified-operator-bundle
build-certified-operator-bundle:yq python operator-sdk verify_bundle_creation_parameters create_certified_operator_bundle_directory update_image_digest_in_operator_bundle update_operator_version_and_olm_skipRange_in_operator_bundle update_cluster_permissions_in_operator_bundle update_release_channel_in_operator_bundle validate_operator_bundle
## Verify bundle creation parameters
.PHONY: verify_bundle_creation_parameters
verify_bundle_creation_parameters: verify_operator_release_versions verify_operator_release_channel verify_stable_operator_release_version verify_image_digest_version
## Verify either the operator release version or the operator release version patch is not empty
verify_operator_release_versions:
	@if [ -z "$(OPERATOR_RELEASE_VERSION)" ] || [ -z "$(OPERATOR_RELEASE_VERSION_PATCH)" ]; then \
		echo "Error: The operator release version is empty, cannot proceed with $(OPERATOR_CERTIFIED)-operator bundle release."; \
		exit 1; \
	fi
## verify operator release channel, to allow only valid releases
verify_operator_release_channel:
    ifneq ($(filter $(OPERATOR_RELEASE_CHANNEL),stable beta),$(OPERATOR_RELEASE_CHANNEL))
        $(error Invalid operator release channel parameter - $(OPERATOR_RELEASE_CHANNEL). valid release channels are either "stable" or "beta only".)
    endif
## verify operator release version on stable channel, to avoid multiple releases of same version
verify_stable_operator_release_version:
	if [ "$(OPERATOR_RELEASE_CHANNEL)" = "stable" ]; then \
        echo "Checking if the stable release version $(OPERATOR_RELEASE_VERSION_PATCH) exists..."; \
         if [ -n "$$(curl -s "$(GITHUB_REPO_URL)" | jq -r 'map(select(.type == "dir" and .name == "$(OPERATOR_RELEASE_VERSION_PATCH)")) | .[].name')" ]; then \
            echo "Error: The operator release version already exists for stable channel: $(OPERATOR_RELEASE_VERSION_PATCH)."; \
            exit 1; \
        fi; \
    fi
## verify if the version field value is present in the image to proceed, if empty exit the execution
verify_image_digest_version:
	@echo "Verify Image Digest version field value"
	$(eval OPERATOR_IMAGE_RELEASE_VERSION := $(if $(filter $(OPERATOR_BETA_RELEASE_FILTER),$(OPERATOR_RELEASE_CHANNEL)),$(OPERATOR_RELEASE_VERSION)-$(OPERATOR_BETA_RELEASE_VERSION_SNAPSHOT),$(OPERATOR_RELEASE_VERSION)))
	docker pull $(REGISTRY)/$(OPERATOR_NAME):$(OPERATOR_IMAGE_RELEASE_VERSION)
	version=$$(docker inspect $(REGISTRY)/$(OPERATOR_NAME):$(OPERATOR_IMAGE_RELEASE_VERSION) | grep '"version":' | awk '{print $$2}' | tr -d '",'); \
	if [ -z "$$version" ]; then \
		echo "Error: Image digest version field is empty, cannot procced with $(OPERATOR_CERTIFIED)-operator bundle release."; \
		exit 1; \
	elif [ "$$version" != "$(OPERATOR_IMAGE_RELEASE_VERSION)" ]; then \
		echo "Error: Image digest version field: ($$version) does not match operator release version: ($(OPERATOR_IMAGE_RELEASE_VERSION))."; \
		exit 1; \
	else \
		echo "Image digest validation successful, proceeding with next steps."; \
	fi

## create certified operator bundle dir and copy the base files to update the clusterserviceversion and metadata contents as required for releasing
create_certified_operator_bundle_directory:
	@echo "Creating certified operator bundle files for $(OPERATOR_CERTIFIED) clusterserviceversion..."
	mkdir -p $(OPERATOR_BUNDLE_DIR)/manifests/
	cp $(OPERATOR_CRD_FILE_PATH) $(OPERATOR_BUNDLE_DIR)/manifests/kubeturbos.charts.helm.k8s.io.crd.yaml
	cp $(OPERATOR_BUNDLE_CONFIG_DIR)/manifests/bases/$(OPERATOR_CERTIFIED).clusterserviceversion.yaml $(OPERATOR_BUNDLE_DIR)/manifests/$(OPERATOR_CERTIFIED).clusterserviceversion.yaml
	mkdir -p $(OPERATOR_BUNDLE_DIR)/metadata/
	cp $(OPERATOR_BUNDLE_CONFIG_DIR)/manifests/bases/annotations.yaml $(OPERATOR_BUNDLE_DIR)/metadata/annotations.yaml
	@echo "$(OPERATOR_CERTIFIED)-operator bundle directory created successfully."

## update image digest key
update_image_digest_in_operator_bundle:
	@echo "Updating image digest in $(OPERATOR_CERTIFIED)-clusterserviceversion..."
	$(eval OPERATOR_IMAGE_RELEASE_VERSION := $(if $(filter $(OPERATOR_BETA_RELEASE_FILTER),$(OPERATOR_RELEASE_CHANNEL)),$(OPERATOR_RELEASE_VERSION)-$(OPERATOR_BETA_RELEASE_VERSION_SNAPSHOT),$(OPERATOR_RELEASE_VERSION)))
	digest=$$(docker inspect --format='{{index .RepoDigests 0}}' $(REGISTRY)/$(OPERATOR_NAME):$(OPERATOR_IMAGE_RELEASE_VERSION) | awk -F@ '{print $$2}'); \
	if [ -z "$$digest" ]; then \
		echo "Error: Image digest is empty, cannot proceed with $(OPERATOR_CERTIFIED)-operator bundle release."; \
		exit 1; \
	else \
	    $(YQ) eval -i '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image |= sub("sha256:.*", "'$$digest'") | .spec.relatedImages[0].image |= sub("sha256:.*", "'$$digest'")' \
		$(OPERATOR_BUNDLE_DIR)/manifests/$(OPERATOR_CERTIFIED).clusterserviceversion.yaml; \
	    echo "$(OPERATOR_CERTIFIED)-clusterserviceversion image digest updated."; \
    fi

## Update release version, olm.skipRange, and check beta release version
update_operator_version_and_olm_skipRange_in_operator_bundle:
	@echo "Checking the minor version in the beta release channel, and updating release versions as well as the olm.skipRange value in $(OPERATOR_CERTIFIED)-clusterserviceversion..."
## Check the release candidate version for the beta channel; if it exists, increment the minor verion(beta.x).
	$(eval OPERATOR_VERSION_BETA := $(shell \
		OPERATOR_BETA_RELEASE_MINOR_VERSION=$$(curl -s "$(GITHUB_REPO_URL)" | \
		jq -r 'map(select(.type == "dir")) | .[].name | match("$(OPERATOR_RELEASE_VERSION_PATCH)-beta\\.[0-9]+") | try .string catch "0"' | \
		awk -F'.' 'BEGIN{max=0} {n=substr($$0, index($$0, "beta.")+5)+0; if (n>max) max=n} END{print max}'); \
		if [ -z "$$OPERATOR_BETA_RELEASE_MINOR_VERSION" ]; then \
			OPERATOR_BETA_RELEASE_MINOR_VERSION=1; \
		else \
			OPERATOR_BETA_RELEASE_MINOR_VERSION=$$(($$OPERATOR_BETA_RELEASE_MINOR_VERSION + 1)); \
		fi; \
		OPERATOR_VERSION_BETA=beta.$$OPERATOR_BETA_RELEASE_MINOR_VERSION; \
		echo "$$OPERATOR_VERSION_BETA" \
	))
	$(eval OPERATOR_RELEASE_CHANNEL_VERSION := $(if $(filter $(OPERATOR_BETA_RELEASE_FILTER),$(OPERATOR_RELEASE_CHANNEL)),$(OPERATOR_RELEASE_VERSION_PATCH)-$(OPERATOR_VERSION_BETA),$(OPERATOR_RELEASE_VERSION_PATCH)))
	$(eval OLMRANGE_LOWER_BOUND := $(if $(filter $(OPERATOR_BETA_RELEASE_FILTER),$(OPERATOR_RELEASE_CHANNEL)),$(OPERATOR_OLM_INCLUSIVE_RANGE_VERSION)-$(OPERATOR_OLM_INCLUSIVE_BETA_VERSION),$(OPERATOR_OLM_INCLUSIVE_RANGE_VERSION)))
	$(eval OLMRANGE_UPPER_BOUND := $(if $(filter $(OPERATOR_BETA_RELEASE_FILTER),$(OPERATOR_RELEASE_CHANNEL)),$(OPERATOR_RELEASE_VERSION_PATCH)-$(OPERATOR_VERSION_BETA),$(OPERATOR_RELEASE_VERSION_PATCH)))
## Update the release version in clusterserviceversion
	$(YQ) eval -i '.metadata.name |= sub("kubeturbo-operator.v.*", "kubeturbo-operator.v$(OPERATOR_RELEASE_CHANNEL_VERSION)") | .spec.version = "$(OPERATOR_RELEASE_CHANNEL_VERSION)"' \
	$(OPERATOR_BUNDLE_DIR)/manifests/$(OPERATOR_CERTIFIED).clusterserviceversion.yaml
## Update skipRange based on inclusive and exclusive release versions set in clusterserviceversion
	$(YQ) eval -i '.metadata.annotations."olm.skipRange" |= sub(">=[^<]+", ">=$(OLMRANGE_LOWER_BOUND)") | .metadata.annotations."olm.skipRange" |= sub("<[^<]+", " <$(OLMRANGE_UPPER_BOUND)")' \
	$(OPERATOR_BUNDLE_DIR)/manifests/$(OPERATOR_CERTIFIED).clusterserviceversion.yaml
	@echo "$(OPERATOR_CERTIFIED)-clusterserviceversion release version, olm.skipRange are updated successfully."

## update cluster permissions roles
update_cluster_permissions_in_operator_bundle:
	@echo "Updating cluster permissions roles in $(OPERATOR_CERTIFIED)-clusterserviceversion..."
	$(PYTHON) $(OPERATOR_BUNDLE_CONFIG_DIR)/manifests/cluster_permissions_automation.py \
	$(CLUSTER_PERMISSION_ROLE_YAML_FILE_PATH) \
	$(CERTIFIED_OPERATOR_CLUSTER_SERVICE_VERSION_YAML_FILE_PATH)
	@echo "$(OPERATOR_CERTIFIED)-clusterserviceversion cluster permissions roles updated successfully."

## update release channel
update_release_channel_in_operator_bundle:
	@echo "Updating release channel in  $(OPERATOR_CERTIFIED)-annotations..."
	$(YQ) eval -i '.annotations."operators.operatorframework.io.bundle.channels.v1" = "$(OPERATOR_RELEASE_CHANNEL)"' \
	$(OPERATOR_BUNDLE_DIR)/metadata/annotations.yaml
	@echo "$(OPERATOR_CERTIFIED)-annotations release channel updated successfully."

## validate operator bundle
validate_operator_bundle:
	@echo "Validating $(OPERATOR_BUNDLE_DIR) through Operator SDK:$(OPERATOR_SDK_VERSION) for compliance and correctness..."
	$(OPERATOR_SDK) bundle validate ./$(OPERATOR_BUNDLE_DIR)


.PHONY: opm
OPM = $(LOCALBIN)/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

## Custom targets

# Create a kind cluster if not exist
.PHONY: create-kind-cluster
create-kind-cluster: kind
	$(KIND) get clusters | grep "^$(KIND_CLUSTER)$$" || \
	$(KIND) create cluster \
		--name $(KIND_CLUSTER) \
		--kubeconfig $(KIND_KUBECONFIG) \
		--config ./scripts/multi-node-kind-cluster.yaml

.PHONY: describe-vars
describe-vars:
	# REGISTRY:        $(REGISTRY)
	# OPERATOR_NAME:   $(OPERATOR_NAME)
	# VERSION:         $(VERSION)
	# NAMESPACE:       $(NAMESPACE)
	# KUBECTL:         $(KUBECTL)
	# KIND:            $(KIND)
	# KIND_CLUSTER:    $(KIND_CLUSTER)
	# KIND_KUBECONFIG: $(KIND_KUBECONFIG)

.PHONY: go-mod-tidy
go-mod-tidy: ## Add missing and remove unused Go modules
	go mod tidy

.PHONY: go-generate
go-generate: ## Run go code generation
	go get github.com/maxbrunsfeld/counterfeiter/v6
	go generate ./...

.PHONY: git-check-generated-items
git-check-generated-items: manifests generate export_yaml run-shellcheck
	@echo "Checking if all 'make manifests generate export_yaml' items are commited ..."
	$(eval result = $(shell git status --untracked-files=all | grep -oE '\s+(config|api|deploy)(/[a-zA-Z0-9._-]+)*' | sed -e 's/\t//g' -e 's/ //g'))
	@if [[ -n "$(result)" ]] ; then \
		echo "Here are some uncommitted auto-generated files:"; \
		for it in $(result); do echo "* $$it"; done; \
		echo "Please run 'make git-check-generated-items' before pushing the branch."; \
		exit 1; \
	fi
	@echo "Done for checking auto-generated files!"

.PHONY: helm
helm: $(LOCALBIN) ## Download helm locally if necessary.
	@if ! command -v $(HELM) >/dev/null 2>&1 ; then \
		OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
		curl -Lo "helm-v3.16.1-$${OS}-$${ARCH}.tar.gz" "https://get.helm.sh/helm-v3.16.1-$${OS}-$${ARCH}.tar.gz" && \
		tar -zxvf "helm-v3.16.1-$${OS}-$${ARCH}.tar.gz" && \
		mv $${OS}-$${ARCH}/helm $(LOCALBIN)/ && \
        rm -f "helm-v3.16.1-$${OS}-$${ARCH}.tar.gz" && \
        rm -rf "$${OS}-$${ARCH}/"; \
	fi
	$(HELM) version

HELM_LINTER := docker run --rm --workdir=/data --volume $(shell pwd):/data  quay.io/helmpack/chart-testing:v3.11.0 ct

.PHONY:helm-lint
helm-lint:
	$(HELM_LINTER) lint --charts deploy/kubeturbo --validate-maintainers=false --target-branch staging

.PHONY:public-repo-update
public-repo-update: helm
	@if [[ "$(VERSION)" =~ ^[0-9]+\.[0-9]+\.[0-9]+$$ ]] ; then \
		./scripts/public_repo_update.sh ${VERSION}; \
	fi

.PHONY: helm-test
helm-test: helm-lint helm create-kind-cluster kubectl
	VERSION=${DEFAULT_KUBETURBO_VERSION} KUBECONFIG=${KIND_KUBECONFIG} ./scripts/kubeturbo_deployment_helm_test.sh

.PHONY: yaml-test
yaml-test: create-kind-cluster kubectl
	VERSION=${DEFAULT_KUBETURBO_VERSION} KUBECONFIG=${KIND_KUBECONFIG} ./scripts/kubeturbo_deployment_yaml_test.sh
# Minimum severity of errors to consider (error, warning, info, style)
SHELLCHECK_SEVERITY ?= "warning"
SHELLCHECK_FOLDER ?= "scripts"
SHELLCHECK ?= $(LOCALBIN)/shellcheck

.PHONY: shellcheck
shellcheck: $(SHELLCHECK) ## Download shellcheck locally if necessary.

# shellcheck tool refers to https://github.com/koalaman/shellcheck
$(SHELLCHECK): $(LOCALBIN)
	LOCALBIN=$(LOCALBIN) sh ./scripts/download_tools.sh

.PHONY: run-shellcheck
run-shellcheck: shellcheck ## Run shellcheck against bash files to check the error syntax
	@echo "Running shellcheck against all *.sh files under the $(SHELLCHECK_FOLDER) folder..."
	@echo "Note: In case of failure, please run 'make run-shellcheck' for more info!"
	@find $(SHELLCHECK_FOLDER) -type f -name "*.sh" -exec $(SHELLCHECK) --severity=$(SHELLCHECK_SEVERITY) {} +;
	@echo "Done for shellcheck scan!"

.PHONY: run-shellcheck-docker ## In case shellcheck cil not working then use docker image for instead
run-shellcheck-docker: ## Run shellcheck against bash files to check the error syntax using docker image
	@echo "Running shellcheck against all *.sh files under the $(SHELLCHECK_FOLDER) folder..."
	@find $(SHELLCHECK_FOLDER) -type f -name "*.sh" -exec docker run --rm -v "$(shell pwd):/mnt" koalaman/shellcheck:stable --severity=$(SHELLCHECK_SEVERITY) {} +;
	@echo "Done for shellcheck scan!"

.PHONY: CI
CI: docker-build

CD: docker-buildx
