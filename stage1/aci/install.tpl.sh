# this is a template file that will be expanded by rkt.mk
# in order to produce an install script.
# variables that are expanded have the syntax: {{NAME}}
# and are explicitly passed to write-template function call

set -e

${INSTALL} -m 0644 {{PWD}}/aci-manifest ${ACI}/manifest

${INSTALL} -d "${ROOT}/etc"
echo "rkt" > "${ROOT}/etc/os-release"

# parent dir for the stage2 bind mounts
${INSTALL} -d "${ROOT}/opt/stage2"

# dir for result code files
${INSTALL} -d "${ROOT}/rkt/status"

# dir for env files
${INSTALL} -d "${ROOT}/rkt/env"
