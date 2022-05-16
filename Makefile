PKG=github.com/cyverse/gocommands
VERSION=v0.1.0
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
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/goinit ./cmd/goinit/goinit.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gols ./cmd/gols/gols.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/goget ./cmd/goget/goget.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/goput ./cmd/goput/goput.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gocd ./cmd/gocd/gocd.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gopwd ./cmd/gopwd/gopwd.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gomv ./cmd/gomv/gomv.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gocp ./cmd/gocp/gocp.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gorm ./cmd/gorm/gorm.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gormdir ./cmd/gormdir/gormdir.go
	CGO_ENABLED=0 go build -ldflags=${LDFLAGS} -o bin/gomkdir ./cmd/gomkdir/gomkdir.go
	
