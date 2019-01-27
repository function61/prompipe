#!/bin/bash -eu

source /build-common.sh

BINARY_NAME="prompipe"
COMPILE_IN_DIRECTORY="cmd/prompipe"
BINTRAY_PROJECT="function61/prompipe"

standardBuildProcess
