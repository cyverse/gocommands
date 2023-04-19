#!/bin/bash
#
# This script updates variables in $1 and output to $2.
#
# It update following variables
#
# NAME                                 update it with "name" value in package_info.json
# VERSION                              update it with "version" value in package_info.json
# SOURCE_SHA256                        update it with source tarball's sha256sum
# RELEASE_BINARY_OSX_AMD64_MD5         update it with binary package's md5sum
# RELEASE_BINARY_OSX_ARM64_MD5         update it with binary package's md5sum
# RELEASE_BINARY_LINUX_386_MD5         update it with binary package's md5sum
# RELEASE_BINARY_LINUX_AMD64_MD5       update it with binary package's md5sum
# RELEASE_BINARY_LINUX_ARM_MD5         update it with binary package's md5sum
# RELEASE_BINARY_LINUX_ARM64_MD5       update it with binary package's md5sum
# RELEASE_BINARY_WINDOWS_386_MD5       update it with binary package's md5sum
# RELEASE_BINARY_WINDOWS_AMD64_MD5     update it with binary package's md5sum


NAME=$(jq -r .name package_info.json)
VERSION=$(jq -r .version package_info.json)
CUR_RELEASE_URL=https://github.com/cyverse/gocommands/releases/download/v${VERSION}
CUR_SOURCE_RELEASE_URL=https://github.com/cyverse/gocommands/archive/refs/tags

SOURCE_SHA256=
RELEASE_BINARY_OSX_AMD64_MD5=
RELEASE_BINARY_OSX_ARM64_MD5=
RELEASE_BINARY_LINUX_386_MD5=
RELEASE_BINARY_LINUX_AMD64_MD5=
RELEASE_BINARY_LINUX_ARM_MD5=
RELEASE_BINARY_LINUX_ARM64_MD5=
RELEASE_BINARY_WINDOWS_386_MD5=
RELEASE_BINARY_WINDOWS_AMD64_MD5=

get_source_sha256()
{
  local tarURL="${CUR_SOURCE_RELEASE_URL}/v${VERSION}.tar.gz"
  printf '%s' $(curl -sL ${tarURL} | sha256sum | awk '{print $1}')
}

get_release_md5()
{
  #$1 = os-arch
  local platform=$1
  local tarURL=
  local md5URL=

  if [[ $platform == *"windows"* ]]; then
    tarURL="${CUR_RELEASE_URL}/gocmd-v${VERSION}-${platform}.zip"
    md5URL="${tarURL}.md5"
  else
    tarURL="${CUR_RELEASE_URL}/gocmd-v${VERSION}-${platform}.tar.gz"
    md5URL="${tarURL}.md5"
  fi

  printf '%s' $(curl -sL ${md5URL})
}

main()
{
  SOURCE_SHA256=$(get_source_sha256)
  RELEASE_BINARY_OSX_AMD64_MD5=$(get_release_md5 "darwin-amd64")
  RELEASE_BINARY_OSX_ARM64_MD5=$(get_release_md5 "darwin-arm64")
  RELEASE_BINARY_LINUX_386_MD5=$(get_release_md5 "linux-386")
  RELEASE_BINARY_LINUX_AMD64_MD5=$(get_release_md5 "linux-amd64")
  RELEASE_BINARY_LINUX_ARM_MD5=$(get_release_md5 "linux-arm")
  RELEASE_BINARY_LINUX_ARM64_MD5=$(get_release_md5 "linux-arm64")
  RELEASE_BINARY_WINDOWS_386_MD5=$(get_release_md5 "windows-386")
  RELEASE_BINARY_WINDOWS_AMD64_MD5=$(get_release_md5 "windows-amd64")

  expand_tmpl $1 > $2
}


# escapes / and \ for sed script
escape()
{
  local var="$*"

  # Escape \ first to avoid escaping the escape character, i.e. avoid / -> \/ -> \\/
  var="${var//\\/\\\\}"

  printf '%s' "${var//\//\\/}"
}


expand_tmpl()
{
  cat <<EOF | sed --file - "$1"
s/\$VERSION/$(escape $VERSION)/g
s/\$RELEASE_URL/$(escape $RELEASE_URL)/g
s/\$SOURCE_SHA256/$(escape $SOURCE_SHA256)/g
s/\$RELEASE_BINARY_OSX_AMD64_MD5/$(escape $RELEASE_BINARY_OSX_AMD64_MD5)/g
s/\$RELEASE_BINARY_OSX_ARM64_MD5/$(escape $RELEASE_BINARY_OSX_ARM64_MD5)/g
s/\$RELEASE_BINARY_LINUX_386_MD5/$(escape $RELEASE_BINARY_LINUX_386_MD5)/g
s/\$RELEASE_BINARY_LINUX_AMD64_MD5/$(escape $RELEASE_BINARY_LINUX_AMD64_MD5)/g
s/\$RELEASE_BINARY_LINUX_ARM_MD5/$(escape $RELEASE_BINARY_LINUX_ARM_MD5)/g
s/\$RELEASE_BINARY_LINUX_ARM64_MD5/$(escape $RELEASE_BINARY_LINUX_ARM64_MD5)/g
s/\$RELEASE_BINARY_WINDOWS_386_MD5/$(escape $RELEASE_BINARY_WINDOWS_386_MD5)/g
s/\$RELEASE_BINARY_WINDOWS_AMD64_MD5/$(escape $RELEASE_BINARY_WINDOWS_AMD64_MD5)/g
EOF
}

set -e

main $1 $2
