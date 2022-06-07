#!/bin/bash

main()
{
  mkdir -p $1

  expand_tmpl install.sh.template > $1/install.sh
  chmod 700 $1/install.sh

  expand_tmpl uninstall.sh.template > $1/uninstall.sh
  chmod 700 $1/uninstall.sh
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
  local SUBCOMMAND_NAMES=$(ls -1 ../cmd/subcmd | sed -e 's/\.go$//' | sed -e 's/^go//')

  cat <<EOF | sed --file - $1
s/\$SUBCOMMAND_NAMES/$(escape $SUBCOMMAND_NAMES)/g
EOF
}


set -e
main "$@"
