PKG=github.com/cyverse/gocommands
VERSION=v$(shell jq -r .version package_info.json)
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X '${PKG}/commons.clientVersion=${VERSION}' -X '${PKG}/commons.gitCommit=${GIT_COMMIT}' -X '${PKG}/commons.buildDate=${BUILD_DATE}'"
GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)

.EXPORT_ALL_VARIABLES:

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gocmd ./cmd/*.go

.PHONY: update_version
update_version:
	./tools/update-pkginfo.sh README.md.template README.md
	./tools/update-pkginfo.sh conda/meta.yaml.template conda/meta.yaml
	./tools/update-pkginfo.sh homebrew/gocommands.rb.template homebrew/gocommands.rb
	cp LICENSE conda/LICENSE


