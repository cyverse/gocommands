#!/bin/bash

main()
{
  mkdir -p $1

  local SUBCOMMAND_NAMES=$(ls -1 ../cmd/subcmd | sed -e 's/\.go$//' | sed -e 's/^go//')
  local SUBCOMMANDS=($SUBCOMMAND_NAMES)
  
  for i in "${SUBCOMMANDS[@]}"
  do
    echo -e "#!/bin/bash\nbaseDir=\$(dirname \"\$0\")\n\$baseDir/gocmd $i \$@" > $1/go$i
    chmod 700 $1/go$i
  done  
}

set -e
main "$@"
