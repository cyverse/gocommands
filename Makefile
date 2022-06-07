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
	

.PHONY: build-release
build-release:
# 	i386_linux
	mkdir -p release/i386_linux
	cd install && ./prep-install-script.sh ../release/i386_linux && cd ..
	cd build && ./build.sh linux 386 ../release/i386_linux ${LDFLAGS} && cd ..
	cd release/i386_linux && tar cf gocommands_i386_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/i386_linux

# 	amd64_linux
	mkdir -p release/amd64_linux
	cd install && ./prep-install-script.sh ../release/amd64_linux && cd ..
	cd build && ./build.sh linux amd64 ../release/amd64_linux ${LDFLAGS} && cd ..
	cd release/amd64_linux && tar cf gocommands_amd64_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/amd64_linux

# 	arm_linux
	mkdir -p release/arm_linux
	cd install && ./prep-install-script.sh ../release/arm_linux && cd ..
	cd build && ./build.sh linux arm ../release/arm_linux ${LDFLAGS} && cd ..
	cd release/arm_linux && tar cf gocommands_arm_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/arm_linux

# 	arm64_linux
	mkdir -p release/arm64_linux
	cd install && ./prep-install-script.sh ../release/arm64_linux && cd ..
	cd build && ./build.sh linux arm64 ../release/arm64_linux ${LDFLAGS} && cd ..
	cd release/arm64_linux && tar cf gocommands_arm64_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/arm64_linux

# 	amd64_darwin
	mkdir -p release/amd64_darwin
	cd install && ./prep-install-script.sh ../release/amd64_darwin && cd ..
	cd build && ./build.sh darwin amd64 ../release/amd64_darwin ${LDFLAGS} && cd ..
	cd release/amd64_darwin && tar cf gocommands_amd64_darwin_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/amd64_darwin

# 	arm64_darwin
	mkdir -p release/arm64_darwin
	cd install && ./prep-install-script.sh ../release/arm64_darwin && cd ..
	cd build && ./build.sh darwin arm64 ../release/arm64_darwin ${LDFLAGS} && cd ..
	cd release/arm64_darwin && tar cf gocommands_arm64_darwin_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/arm64_darwin

# 	i386_windows
	mkdir -p release/i386_windows
	cd install && ./prep-install-script.sh ../release/i386_windows && cd ..
	cd build && ./build.sh windows 386 ../release/i386_windows ${LDFLAGS} && cd ..
	cd release/i386_windows && tar cf gocommands_i386_windows_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/i386_windows

# 	amd64_windows
	mkdir -p release/amd64_windows
	cd install && ./prep-install-script.sh ../release/amd64_windows && cd ..
	cd build && ./build.sh windows amd64 ../release/amd64_windows ${LDFLAGS} && cd ..
	cd release/amd64_windows && tar cf gocommands_amd64_windows_${VERSION}.tar * && mv *.tar .. && cd ../..
	rm -rf release/amd64_windows


