#!/bin/bash

HOST=$1
TOTAL=$2

COUNTER=1 ; while [[ ${COUNTER} -le ${TOTAL} ]] ; do \
    echo ${COUNTER} ; \
    KEY=$RANDOM ; \
    VALUE=$RANDOM ; \
    curl -X POST -d "${VALUE}" -s ${HOST}/${KEY}
    COUNTER=`expr ${COUNTER} + 1` ; \
done
