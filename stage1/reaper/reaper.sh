#!/usr/bin/bash
shopt -s nullglob

SYSCTL=/usr/bin/systemctl

app=$1
status=$(${SYSCTL} show --property ExecMainStatus "${app}.service")
echo "${status#*=}" > "/rkt/status/$app"
