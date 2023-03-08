PKG=github.com/cyverse/gocommands
VERSION=v0.6.1
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


.PHONY: test-release
test-release:
	rm -rf release

# 	amd64_linux
	mkdir -p release/amd64_linux
	cd install && ./prep-install-script.sh ../release/amd64_linux && cd ..
	cd install && ./prep-shortcut-script.sh ../release/amd64_linux && cd ..
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags=${LDFLAGS} -o release/amd64_linux/gocmd cmd/*.go
	cd release/amd64_linux && tar cf gocommands_amd64_linux_${VERSION}.tar * && mv *.tar .. && cd ../..

.PHONY: test-win
test-win:
	rm -rf release

	mkdir -p release/amd64_windows
	CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build -ldflags=${LDFLAGS} -o release/amd64_windows/gocmd.exe cmd/*.go


.PHONY: build-release
build-release:
	rm -rf release

# 	i386_linux
	mkdir -p release/i386_linux
	cd install && ./prep-install-script.sh ../release/i386_linux && cd ..
	cd install && ./prep-shortcut-script.sh ../release/i386_linux && cd ..
	CGO_ENABLED=0 GOARCH=386 GOOS=linux go build -ldflags=${LDFLAGS} -o release/i386_linux/gocmd cmd/*.go
	cd release/i386_linux && tar cf gocommands_i386_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	cd release/i386_linux && tar cf gocommands_i386_linux_${VERSION}_portable.tar gocmd && mv *.tar .. && cd ../..
	rm -rf release/i386_linux

# 	amd64_linux
	mkdir -p release/amd64_linux
	cd install && ./prep-install-script.sh ../release/amd64_linux && cd ..
	cd install && ./prep-shortcut-script.sh ../release/amd64_linux && cd ..
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags=${LDFLAGS} -o release/amd64_linux/gocmd cmd/*.go
	cd release/amd64_linux && tar cf gocommands_amd64_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	cd release/amd64_linux && tar cf gocommands_amd64_linux_${VERSION}_portable.tar gocmd && mv *.tar .. && cd ../..
	rm -rf release/amd64_linux

# 	arm_linux
	mkdir -p release/arm_linux
	cd install && ./prep-install-script.sh ../release/arm_linux && cd ..
	cd install && ./prep-shortcut-script.sh ../release/arm_linux && cd ..
	CGO_ENABLED=0 GOARCH=arm GOOS=linux go build -ldflags=${LDFLAGS} -o release/arm_linux/gocmd cmd/*.go
	cd release/arm_linux && tar cf gocommands_arm_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	cd release/arm_linux && tar cf gocommands_arm_linux_${VERSION}_portable.tar gocmd && mv *.tar .. && cd ../..
	rm -rf release/arm_linux

# 	arm64_linux
	mkdir -p release/arm64_linux
	cd install && ./prep-install-script.sh ../release/arm64_linux && cd ..
	cd install && ./prep-shortcut-script.sh ../release/arm64_linux && cd ..
	CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -ldflags=${LDFLAGS} -o release/arm64_linux/gocmd cmd/*.go
	cd release/arm64_linux && tar cf gocommands_arm64_linux_${VERSION}.tar * && mv *.tar .. && cd ../..
	cd release/arm64_linux && tar cf gocommands_arm64_linux_${VERSION}_portable.tar gocmd && mv *.tar .. && cd ../..
	rm -rf release/arm64_linux

# 	amd64_darwin
	mkdir -p release/amd64_darwin
	cd install && ./prep-install-script.sh ../release/amd64_darwin && cd ..
	cd install && ./prep-shortcut-script.sh ../release/amd64_darwin && cd ..
	CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build -ldflags=${LDFLAGS} -o release/amd64_darwin/gocmd cmd/*.go
	cd release/amd64_darwin && tar cf gocommands_amd64_darwin_${VERSION}.tar * && mv *.tar .. && cd ../..
	cd release/amd64_darwin && tar cf gocommands_amd64_darwin_${VERSION}_portable.tar gocmd && mv *.tar .. && cd ../..
	rm -rf release/amd64_darwin

# 	arm64_darwin
	mkdir -p release/arm64_darwin
	cd install && ./prep-install-script.sh ../release/arm64_darwin && cd ..
	cd install && ./prep-shortcut-script.sh ../release/arm64_darwin && cd ..
	CGO_ENABLED=0 GOARCH=arm64 GOOS=darwin go build -ldflags=${LDFLAGS} -o release/arm64_darwin/gocmd cmd/*.go
	cd release/arm64_darwin && tar cf gocommands_arm64_darwin_${VERSION}.tar * && mv *.tar .. && cd ../..
	cd release/arm64_darwin && tar cf gocommands_arm64_darwin_${VERSION}_portable.tar gocmd && mv *.tar .. && cd ../..
	rm -rf release/arm64_darwin

# 	i386_windows
	mkdir -p release/i386_windows
	CGO_ENABLED=0 GOARCH=386 GOOS=windows go build -ldflags=${LDFLAGS} -o release/i386_windows/gocmd.exe cmd/*.go
	cd release/i386_windows && zip gocommands_i386_windows_${VERSION}.zip * && mv *.zip .. && cd ../..
	cd release/i386_windows && zip gocommands_i386_windows_${VERSION}_portable.zip gocmd.exe && mv *.zip .. && cd ../..
	rm -rf release/i386_windows

# 	amd64_windows
	mkdir -p release/amd64_windows
	CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build -ldflags=${LDFLAGS} -o release/amd64_windows/gocmd.exe cmd/*.go
	cd release/amd64_windows && zip gocommands_amd64_windows_${VERSION}.zip * && mv *.zip .. && cd ../..
	cd release/amd64_windows && zip gocommands_amd64_windows_${VERSION}_portable.zip gocmd.exe && mv *.zip .. && cd ../..
	rm -rf release/amd64_windows
