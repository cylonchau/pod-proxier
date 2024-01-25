#!/usr/bin/env bash

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/..; pwd)
CMD=$0

set -e
START_TIME=`date +%s`
export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/usr/local/go/bin:/usr/local/go/sbin

UNKOWNPARAMETER=101
MALFORMEDPARAMTER=110

function USAGE(){
	cat <<EOF
Usage: ${CMD} <command>
Commands:
   Some <commands> take arguments or -h for usage.
     <1.19.10>       Kubernetes version
EOF
}

function DECIDE_VERSION(){
    echo ${PROJECT_ROOT}/hack/mod.sh ${1}
    bash -x ${PROJECT_ROOT}/hack/mod.sh ${1}
    go mod
}
function MAIN(){
	if [[ $# == 0 ]]; then
		USAGE && exit ${MALFORMEDPARAMTER}
	fi
	OPERATION=$1; shift
	DECIDE_VERSION ${OPERATION}
	exit 0
}
MAIN $1