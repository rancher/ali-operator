.PHONY: generate-crd
generate-crd:
	go generate main.go

.PHONY: generate
generate:
	$(MAKE) generate-crd
