#!/usr/bin/bash
shopt -s nullglob

SYSCTL=/usr/bin/systemctl

cd /opt/stage2
for app in *; do
        status=$(${SYSCTL} show --property ExecMainStatus "${app}.service")
        echo "${status#*=}" > "/rkt/status/$app"
done

${SYSCTL} halt --force
