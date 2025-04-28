PKG=github.com/cyverse/gocommands
VERSION=v$(shell jq -r .version package_info.json)
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X '${PKG}/commons.clientVersion=${VERSION}' -X '${PKG}/commons.gitCommit=${GIT_COMMIT}' -X '${PKG}/commons.buildDate=${BUILD_DATE}'"
GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
DOCKER_IMAGE=cyverse/gocmd
DOCKERFILE=docker/Dockerfile

.EXPORT_ALL_VARIABLES:

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gocmd ./cmd/gocmd.go

.PHONY: version
version:
	./tools/update-pkginfo.sh homebrew/gocommands.rb.template homebrew/gocommands.rb
	./tools/create-version.sh VERSION.txt

.PHONY: thirdparty_licenses
thirdparty_licenses:
	go-licenses report ./cmd --template thirdparty_licenses.template
	
.PHONY: image
image:
	docker build -t $(DOCKER_IMAGE):${VERSION} -f $(DOCKERFILE) .
	docker tag $(DOCKER_IMAGE):${VERSION} $(DOCKER_IMAGE):latest

.PHONY: push
push: image
	docker push $(DOCKER_IMAGE):${VERSION}
	docker push $(DOCKER_IMAGE):latest