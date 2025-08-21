default: operator

.PHONY: generate-crd
generate-crd:
	go generate main.go

.PHONY: generate
generate:
	$(MAKE) generate-crd

.PHONY: operator
operator:
	CGO_ENABLED=0 go build -ldflags \
            "-X github.com/rancher/ali-operator/pkg/version.GitCommit=$(GIT_COMMIT) \
             -X github.com/rancher/ali-operator/pkg/version.Version=$(TAG)" \
        -o bin/ali-operator .