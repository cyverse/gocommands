#!/bin/bash
#
# This script creates a version file.

VERSION=$(jq -r .version package_info.json)

main()
{
  echo "v$VERSION" | tee $1
}

set -e

main $1
