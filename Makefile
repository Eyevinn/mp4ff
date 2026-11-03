.PHONY: all
all: test check coverage build

.PHONY: build
build: mp4ff-crop mp4ff-decrypt mp4ff-defragment mp4ff-encrypt mp4ff-info mp4ff-mvhevc mp4ff-nallister mp4ff-pslister mp4ff-subslister examples

.PHONY: prepare
prepare:
	go mod tidy

# Building ./cmd/<tool> as a package makes Go embed the version from git, which the tools report.
# The -X flags are for builds from a source archive, without git, where packagers replace the git commands.
.PHONY: mp4ff-crop mp4ff-decrypt mp4ff-defragment mp4ff-encrypt mp4ff-info mp4ff-mvhevc mp4ff-nallister mp4ff-pslister mp4ff-subslister
mp4ff-crop mp4ff-decrypt mp4ff-defragment mp4ff-encrypt mp4ff-info mp4ff-mvhevc mp4ff-nallister mp4ff-pslister mp4ff-subslister:
	go build -ldflags "-X github.com/Eyevinn/mp4ff/internal.commitVersion=$$(git describe --tags HEAD) -X github.com/Eyevinn/mp4ff/internal.commitDate=$$(git log -1 --format=%ct)" -o out/$@ ./cmd/$@

.PHONY: examples
examples: add-sidx combine-segs initcreator multitrack resegmenter segmenter

add-sidx combine-segs initcreator multitrack resegmenter segmenter:
	go build -o examples-out/$@  ./examples/$@

.PHONY: test
test: prepare
	go test ./...

.PHONY: testsum
testsum: prepare
	gotestsum

.PHONY: open-docs
open-docs:
	echo "If needed: go install golang.org/x/pkgsite/cmd/pkgsite@latest"
	pkgsite -http localhost:9999
	# open http://localhost:9999/pkg/github.com/Eyevinn/mp4ff/

.PHONY: coverage
coverage:
	# Ignore (allow) packages without any tests
	go test -coverpkg=./... -coverprofile coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func coverage.out -o coverage.txt
	tail -1 coverage.txt

venv:
	python3 -m venv venv
	venv/bin/pip install --upgrade pip
	venv/bin/pip install pre-commit==4.2.0
	venv/bin/pip install codespell==2.4.1

.PHONY: pre-commit-install
pre-commit-install: venv
	venv/bin/pre-commit install

.PHONY: pre-commit
pre-commit: venv
	venv/bin/pre-commit run --all-files

.PHONY: codespell
codespell: venv
	venv/bin/codespell

.PHONY: check
check: prepare pre-commit codespell

# images/social-preview.png is the social preview of the repository: the logo on a card, at the 1280x640 that
# GitHub asks for. GitHub has no API for it, so a new one is uploaded by hand in the repository settings.
images/social-preview.png: images/social-preview.svg images/mp4ff-logo.svg
	rsvg-convert -o $@ $<

clean:
	rm -f out/*
	rm -r examples-out/*

install: all
	cp out/* $(GOPATH)/bin/

