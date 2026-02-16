#!/bin/bash

while getopts 'w:' opt
do
    case "$opt" in 
    w) 
        WORKDIR="$OPTARG"
        echo "Using working directory: $OPTARG"
        ;;
	*)
	    >&2 echo "Invalid flag: $opt"
		exit 1
    esac
done
shift $((OPTIND-1))

if [[ -n $WORKDIR ]]
then
	if ! [[ -d "$WORKDIR" ]]
	then
	>&2 echo "Error: Could not find working directory at $WORKDIR"
	exit 1
	fi

	cd "$WORKDIR" || exit 1
fi


gandalf ${@}
