# nsenter wrapper @ /trampoline.sh for introspecting the app pid via systemd then entering its namespaces from stage1
# TODO: replace with a go program?  maybe talk directly to sd-bus for the pid?
cat > "${ROOTDIR}/trampoline.sh" <<-'EOF'
#!/usr/bin/bash -e
SYSCTL=/usr/bin/systemctl
NSENTER=/usr/bin/nsenter

[ $# -gt 1 ] || { echo "app imageid and cmd required"; exit 1;}
app=$1
pid=$(${SYSCTL} show --property MainPID "${app}.service")
shift 1
${NSENTER} --mount --uts --ipc --net --pid --root --wd --target "${pid#*=}" "$@"
