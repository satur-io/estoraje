#!/bin/bash

if [[ $1 ]]
then
    HOST=$1
else
    HOST="http://localhost:8000"
fi

RED="\e[31m";
GREEN="\e[32m";
ENDCOLOR="\e[0m";


fails=0;


# Test empty
empty_expected=$(curl -s $HOST/foo);

if [[ $empty_expected != '' || $? != 0 ]]
then
    ((fails+=1));
    echo -e "${RED} Test empty fails ${ENDCOLOR}";
fi

# Test write-read
KEY=$RANDOM
VALUE=$RANDOM

curl -X POST -d "$VALUE" -s $HOST/$KEY

if [[ $? != 0 ]]
then
    ((fails+=1));
    echo -e "${RED} Test write fails ${ENDCOLOR}";
fi

read_value=$(curl -s $HOST/$KEY);

if [[ $read_value != $VALUE || $? != 0 ]]
then
    ((fails+=1));
    echo -e "${RED} Test read fails ${ENDCOLOR}";
fi

# Test remove
curl -X DELETE  -s $HOST/$KEY

if [[ $? != 0 ]]
then
    ((fails+=1));
    echo -e "${RED} Test delete fails (curl error) ${ENDCOLOR}";
fi

read_value=$(curl -s $HOST/$KEY);

if [[ $read_value != "" || $? != 0 ]]
then
    ((fails+=1));
    echo -e "${RED} Test  delete fails (read still working) ${ENDCOLOR}";
fi

# Show results
echo;
echo "-------------------------------------------";
echo;
if [[ $fails -gt 0 ]]
then
    echo -e "${RED} $fails test(s) failed ${ENDCOLOR}";
else
    echo -e "${GREEN} Test passed! ${ENDCOLOR}";
fi
echo;
