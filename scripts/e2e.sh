#!/bin/bash

set -e

find examples -type f -name "Makefile" -exec sh -c '
    dir="{}"
    cd "$(dirname "$dir")" || exit
    make
' \;

find examples -type f -name "Makefile" -exec sh -c '
    dir="{}"
    cd "$(dirname "$dir")" || exit
    make run-in-docker
' \;

find examples_kcl -type f -name "Makefile" -exec sh -c '
    dir="{}"
    cd "$(dirname "$dir")" || exit
    make
' \;
