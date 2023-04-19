#!/bin/bash
#
# This script prints out md5 hashes of release packages


VERSION=$(jq -r .version package_info.json)
CUR_RELEASE_URL=https://github.com/cyverse/gocommands/releases/download/v${VERSION}


main()
{
  for arch in darwin-amd64 darwin-arm64 linux-386 linux-amd64 linux-arm linux-arm64
  do
    local tarURL="${CUR_RELEASE_URL}/gocmd-v${VERSION}-${arch}.tar.gz"
    local md5URL="${tarURL}.md5"
    echo ${tarURL}
    curl -sL ${md5URL}
  done

  for arch in windows-386 windows-amd64
  do
    local tarURL="${CUR_RELEASE_URL}/gocmd-v${VERSION}-${arch}.zip"
    local md5URL="${tarURL}.md5"
    echo ${tarURL}
    curl -sL ${md5URL}
  done
}


set -e

main
