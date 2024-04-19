.PHONY: all
all: test check coverage build

.PHONY: build
build: mp4ff-crop mp4ff-decrypt mp4ff-encrypt mp4ff-info mp4ff-nallister mp4ff-pslister mp4ff-subslister examples

.PHONY: prepare
prepare:
	go mod vendor

mp4ff-crop mp4ff-decrypt mp4ff-encrypt mp4ff-info mp4ff-nallister mp4ff-pslister mp4ff-subslister:
	go build -ldflags "-X github.com/Eyevinn/mp4ff/mp4.commitVersion=$$(git describe --tags HEAD) -X github.com/Eyevinn/mp4ff/mp4.commitDate=$$(git log -1 --format=%ct)" -o out/$@ ./cmd/$@/main.go

.PHONY: examples
examples: initcreator multitrack resegmenter segmenter

initcreator multitrack resegmenter segmenter:
	go build -o examples-out/$@  ./examples/$@

.PHONY: test
test: prepare
	go test ./...

.PHONY: testsum
testsum: prepare
	gotestsum

.PHONY: coverage
coverage:
	# Ignore (allow) packages without any tests
	set -o pipefail
	go test ./... -coverprofile coverage.out
	set +o pipefail
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func coverage.out -o coverage.txt
	tail -1 coverage.txt

.PHONY: check
check: prepare
	golangci-lint run

clean:
	rm -f out/*
	rm -r examples-out/*

install: all
	cp out/* $(GOPATH)/bin/

