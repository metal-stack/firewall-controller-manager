export CGO_ENABLED := 0

# Image URL to use all building/pushing image targets
IMG ?= firewall-controller-manager:latest

SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
BUILDDATE := $(shell date -Iseconds)
VERSION := $(or ${VERSION},$(shell git describe --tags --exact-match 2> /dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD))

CONTROLLER_TOOLS_VERSION ?= v0.14.0
LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

include .env

all: manager

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# clean generated code
.PHONY: clean
clean:
	rm -f bin/*

# Run tests
test: generate manifests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -tags netgo -ldflags "-X 'github.com/metal-stack/v.Version=$(VERSION)' \
								   -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
								   -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
								   -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
						 -o bin/firewall-controller-manager .
	strip bin/firewall-controller-manager

# Run against the mini-lab
deploy: generate fmt vet manifests manager
	kubectl apply -k config
	docker build -f Dockerfile.dev -t fcm .
	kind --name metal-control-plane load docker-image fcm:latest
	kubectl patch deployment -n firewall firewall-controller-manager --patch='{"spec":{"template":{"spec":{"containers":[{"name": "firewall-controller-manager","imagePullPolicy":"IfNotPresent","image":"fcm:latest"}]}}}}'
	kubectl delete pod -n firewall -l app=firewall-controller-manager

# Install CRDs into a cluster
install: manifests
	kustomize build config/crds | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crds | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) +crd:generateEmbeddedObjectMeta=true paths="./..." +output:dir=config/crds
	$(CONTROLLER_GEN) +webhook paths="./..." +output:dir=config/webhooks

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: setup-envtest
setup-envtest: $(ENVTEST)
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
