export CGO_ENABLED := 0

# Image URL to use all building/pushing image targets
IMG ?= firewall-controller-manager:latest

SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
BUILDDATE := $(shell date -Iseconds)
VERSION := $(or ${VERSION},devel)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# clean generated code
.PHONY: clean
clean:
	rm -f bin/*

# Run tests
test:
	@if ! which $(SETUP_ENVTEST) > /dev/null; then echo "setup-envtest needs to be installed. you can use setup-envtest target to achieve this."; exit 1; fi
	KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use --arch=amd64 --bin-dir $(PWD)/bin -p path)" go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -tags netgo -ldflags "-X 'github.com/metal-stack/v.Version=$(VERSION)' \
								   -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
								   -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
								   -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
						 -o bin/firewall-controller-manager main.go
	strip bin/firewall-controller-manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go --cluster-id=abcd --cluster-api-url=https://api.abcd:443 --cert-dir config/examples/certs

# Install CRDs into a cluster
install: manifests
	kustomize build config/crds | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crds | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) +crd:generateEmbeddedObjectMeta=true paths="./..." +output:dir=config/crds
	$(CONTROLLER_GEN) +webhook paths="./..." +output:dir=config/webhooks

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet -copylocks=false ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: setup-envtest
setup-envtest:
ifeq (, $(shell which setup-envtest))
	@{ \
	set -e ;\
	TMP_DIR=$$(mktemp -d) ;\
	cd $$TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest ;\
	rm -rf $$TMP_DIR ;\
	}
SETUP_ENVTEST=$(GOBIN)/setup-envtest
else
SETUP_ENVTEST=$(shell which setup-envtest)
endif
