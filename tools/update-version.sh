#!/bin/bash
#
# This script updates variables in $1 and output to $2.
#
# It update following variables
#
# VERSION                         update it with "version" value in package_info.json

VERSION=$(jq -r .version package_info.json)


main()
{
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
EOF
}

set -e

main $1 $2
