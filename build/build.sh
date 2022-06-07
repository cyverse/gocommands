#!/bin/bash

main()
{
  local BIN_NAMES=$(ls -1 ../cmd)
  local BINS=($BIN_NAMES)

  for i in "${BINS[@]}"
  do
    echo "building $i for $1 $2"
    CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -ldflags="$4" -o $3/$i ../cmd/$i
  done
}

set -e
main "$@"
