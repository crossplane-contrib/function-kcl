#!/bin/bash

set -e

find examples -type f -name "Makefile" -exec sh -c '
    dir="{}"
    cd "$(dirname "$dir")" || exit
    make
' \;
