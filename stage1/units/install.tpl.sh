# this is a template file that will be expanded by rkt.mk
# in order to produce an install script.
# variables that are expanded have the syntax: {{NAME}}
# and are explicitly passed to write-template function call

set -e

# populate the systemd units
${INSTALL} -d -m 0755 ${ROOT}/usr/lib/systemd/system
${INSTALL} -d -m 0755 ${ROOT}/usr/lib/systemd/system/default.target.wants
${INSTALL} -d -m 0755 ${ROOT}/usr/lib/systemd/system/sockets.target.wants

${INSTALL} -m 0644 {{PWD}}/units/default.target ${ROOT}/usr/lib/systemd/system
${INSTALL} -m 0644 {{PWD}}/units/exit-watcher.service ${ROOT}/usr/lib/systemd/system
${INSTALL} -m 0644 {{PWD}}/units/local-fs.target ${ROOT}/usr/lib/systemd/system
${INSTALL} -m 0644 {{PWD}}/units/prepare-app@.service ${ROOT}/usr/lib/systemd/system
${INSTALL} -m 0644 {{PWD}}/units/reaper.service ${ROOT}/usr/lib/systemd/system
${INSTALL} -m 0644 {{PWD}}/units/sockets.target ${ROOT}/usr/lib/systemd/system
