#!/bin/sh

# Derive a minimal rootfs for hosting systemd from a coreos release pxe image
# This is only done when we need to update ../cmd/s1rootfs.go

IMG_RELEASE="444.5.0"
IMG_URL="http://stable.release.core-os.net/amd64-usr/${IMG_RELEASE}/coreos_production_pxe_image.cpio.gz"

req() {
	what=$1

	which "${what}" >/dev/null || { echo "${what} required"; exit 1; }
}

req cpio
req curl
req gcc
req go-bindata
req gpg
req gzip
req install
req mktemp
req tar
req unsquashfs

# coreos gpg signing key
GPG_LONG_ID="50E0885593D2DCB4"
GPG_KEY="-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2

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
Pg==
=LY4G
-----END PGP PUBLIC KEY BLOCK-----"

# gpg verify a file using the provided key
gpg_verify() {
	sigfile=$1	#signature file (assumed to be suffixed form of file to verify)
	key=$2		#signing key
	keyid=$3	#signing key signature

	gpghome=$(mktemp -d)
  keyfile="${PWD}/mkfsroot.gpg.key"
  echo "${key}" > "${keyfile}"
	gpg --homedir="${gpghome}" --batch --quiet --import "${keyfile}"
  rm -f "${keyfile}"
	##### gpg --homedir="${gpghome}" --batch --quiet --import <<<"${key}"
	gpg --homedir="${gpghome}" --batch --trusted-key "${keyid}" --verify "${sigfile}"
	RES=$?

	rm -Rf "${gpghome}"

	return ${RES}
}

# maintain an gpg-verified url cache, assumes signature available @ $url.sig
cache_url() {
	cache=$1
	url=$2
	key=$3
	keyid=$4
	sigfile="${cache}.sig"
	sigurl="${url}.sig"

	# ensure the cache directory exists
	mkdir -p $(dirname "${cache}")

	# verify the cached copy if it exists
  GPG_VERIFY_EXITCODE=$(gpg_verify "${sigfile}" "${key}" "${keyid}" ; echo $?)
  if [ $GPG_VERIFY_EXITCODE -ne 0 ]; then

		# refresh the cache on failure, and verify it again
		curl --location --output "${cache}" "${url}"
		curl --location --output "${sigfile}" "${sigurl}"

		gpg_verify "${sigfile}" "${key}" "${keyid}" || return 1
	fi

	# file $cache exists and can be trusted
}

CACHED_IMG="${PWD}/cache/pxe.img"
WORK="mkroot"
USRFS="usr.squashfs"
ROOTDIR="${WORK}/rootfs"
BINDIR="${WORK}/bins"
USR="rootfs/usr"
FILELIST="filelist.txt"
OUTPUT=${OUTPUT:="../stage0/stage1_rootfs/bin.go"}

# cache pxe image
cache_url "${CACHED_IMG}" "${IMG_URL}" "${GPG_KEY}" "${GPG_LONG_ID}"

[ -e "${WORK}" ] && rm -Rf "${WORK}"

mkdir -p "${ROOTDIR}"

# derive $USRFS from $CACHED_IMG
cd "${WORK}"
gzip -cd "${CACHED_IMG}" | cpio --extract "${USRFS}"

# extra stuff for stage1 which will come/go as things mature (reaper in bash for now)
EXTRAS="bin/bash
	lib64/libreadline.so
	lib64/libreadline.so.6
	lib64/libreadline.so.6.2
	lib64/libncurses.so
	lib64/libncurses.so.5
	lib64/libncurses.so.5.9
	lib64/libdl.so
	lib64/libdl.so.2
	lib64/libdl-2.17.so
	bin/sleep"

# systemd and dependencies
cat > "${FILELIST}" <<-EOF
	${EXTRAS}
	bin/journalctl
	bin/systemctl
	bin/systemd-analyze
	bin/systemd-ask-password
	bin/systemd-cat
	bin/systemd-cgls
	bin/systemd-cgtop
	bin/systemd-coredumpctl
	bin/coredumpctl
	bin/systemd-delta
	bin/systemd-detect-virt
	bin/systemd-inhibit
	bin/systemd-machine-id-setup
	bin/systemd-notify
	bin/systemd-nspawn
	bin/systemd-path
	bin/systemd-run
	bin/systemd-stdio-bridge
	bin/systemd-sysusers
	bin/systemd-tmpfiles
	bin/systemd-tty-ask-password-agent
	lib
	lib64/libattr.so
	lib64/libitm.so
	lib64/libitm.so.1
	lib64/libitm.so.1.0.0
	lib64/libblkid.so
	lib64/libblkid.so.1
	lib64/libblkid.so.1.1.0
	lib64/libuuid.so.1
	lib64/libuuid.so.1.3.0
	lib64/libuuid.so
	lib64/libstdc++.so
	lib64/libstdc++.so.6
	lib64/libstdc++.so.6.0.17
	lib64/libgcc_s.so
	lib64/libgcc_s.so.1
	lib64/librt-2.17.so
	lib64/libz.so.1
	lib64/libc.so
	lib64/libz.so.1.2.8
	lib64/libattr.so.1.1.0
	lib64/libpthread.so.0
	lib64/libz.so
	lib64/libseccomp.so.2.1.1
	lib64/libseccomp.so
	lib64/libpthread.so
	lib64/libcap.so.2.22
	lib64/libpthread-2.17.so
	lib64/libkmod.so.2
	lib64/ld-linux-x86-64.so.2
	lib64/ld-2.17.so
	lib64/librt.so.1
	lib64/libkmod.so
	lib64/libcap.so
	lib64/libc-2.17.so
	lib64/librt.so
	lib64/libseccomp.so.2
	lib64/libattr.so.1
	lib64/libkmod.so.2.2.5
	lib64/libcap.so.2
	lib64/libc.so.6
	lib64/systemd/systemd-backlight
	lib64/systemd/systemd-update-utmp
	lib64/systemd/systemd-vconsole-setup
	lib64/systemd/systemd-journal-remote
	lib64/systemd/systemd-modules-load
	lib64/systemd/systemd-resolved
	lib64/systemd/systemd-bus-proxyd
	lib64/systemd/systemd-ac-power
	lib64/systemd/systemd-bootchart
	lib64/systemd/systemd-initctl
	lib64/systemd/systemd-shutdown
	lib64/systemd/systemd-multi-seat-x
	lib64/systemd/systemd-rfkill
	lib64/systemd/systemd-networkd
	lib64/systemd/systemd-activate
	lib64/systemd/systemd-readahead
	lib64/systemd/systemd-hostnamed
	lib64/systemd/systemd-random-seed
	lib64/systemd/systemd-cgroups-agent
	lib64/systemd/systemd-udevd
	lib64/systemd/systemd-shutdownd
	lib64/systemd/systemd-logind
	lib64/systemd/systemd
	lib64/systemd/systemd-update-done
	lib64/systemd/systemd-machined
	lib64/systemd/systemd-user-sessions
	lib64/systemd/systemd-sysctl
	lib64/systemd/systemd-journald
	lib64/systemd/systemd-timedated
	lib64/systemd/systemd-networkd-wait-online
	lib64/systemd/systemd-localed
	lib64/systemd/systemd-cryptsetup
	lib64/systemd/user-generators
	lib64/systemd/systemd-remount-fs
	lib64/systemd/systemd-coredump
	lib64/systemd/systemd-timesyncd
	lib64/systemd/systemd-socket-proxyd
	lib64/systemd/system-shutdown
	lib64/systemd/systemd-binfmt
	lib64/systemd/systemd-fsck
	lib64/systemd/system-sleep
	lib64/systemd/systemd-sleep
	lib64/systemd/systemd-reply-password
	lib64/systemd/systemd-journal-gatewayd
EOF

unsquashfs -d "${USR}" -ef "${FILELIST}" "${USRFS}" 

cd -

# populate usr/lib/systemd/system with the necessary static stuff
install -d -m 0755 "${ROOTDIR}/usr/lib/systemd/system"

putunit() {
	path="${ROOTDIR}/usr/lib/systemd/system/$1"
	echo "$2" > "${path}"
}

putunit default.target "[Unit]
Description=Rocket apps target
DefaultDependencies=false"

putunit sockets.target "[Unit]
Description=Sockets
DefaultDependencies=false"

putunit local-fs.target "[Unit]
Description=Hook into early systemd for socket-activated systemd instances
DefaultDependencies=false
Requires=sockets.target"

putunit exit-watcher.service "[Unit]
Description=Graceful exit watcher
StopWhenUnneeded=true
DefaultDependencies=false

[Service]
ExecStart=/usr/bin/sleep 9999999999d 
ExecStopPost=/usr/bin/systemctl isolate reaper.service"

putunit reaper.service "[Unit]
Description=Rocket apps reaper
AllowIsolate=true
DefaultDependencies=false

[Service]
ExecStart=/reaper.sh"

install -d -m 0755 "${ROOTDIR}/usr/lib/systemd/system/default.target.wants"
install -d -m 0755 "${ROOTDIR}/usr/lib/systemd/system/sockets.target.wants"

# simple reaper script for collecting the exit statuses of the apps
cat > "${ROOTDIR}/reaper.sh" <<-'EOF'
#!/usr/bin/bash
shopt -s nullglob

SYSCTL=/usr/bin/systemctl

cd /opt/stage2
for app in *; do 
        status=$(${SYSCTL} show --property ExecMainStatus "${app}.service")
        echo "${status#*=}" > "/rkt/status/$app"
done

${SYSCTL} halt --force
EOF
chmod 755 "${ROOTDIR}/reaper.sh"

# LD_PRELOAD shim to trick the sd_booted() "/run/systemd/system" check in systemd-nspawn
# XXX abused further for lockfd retention and container pid recording
gcc -shared -fPIC -x c -pipe -Wl,--no-as-needed -ldl -lc -o ${ROOTDIR}/fakesdboot.so - <<'EOF'
#define _GNU_SOURCE
#include <dlfcn.h>
#include <fcntl.h>
#include <stdarg.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>


/* hack to make systemd-nspawn execute on non-sysd systems:
 * - intercept lstat() so lstat of /run/systemd/system always succeeds and returns a directory
 * - intercept close() to prevent nspawn closing the rkt lock, set it to CLOEXEC instead
 * - intercept syscall(SYS_clone) to record the container's pid
 */

#define ENV_LOCKFD	"RKT_LOCK_FD"
#define PIDFILE_TMP	"pid.tmp"
#define PIDFILE		"pid"

static int (*libc_lxstat)(int, const char *, struct stat *);
static int (*libc_close)(int);
static long (*libc_syscall)(long number, ...);
static int lock_fd = -1;

static __attribute__((constructor)) void wrapper_init(void)
{
	char *env;
	if(env = getenv(ENV_LOCKFD))
		lock_fd = atoi(env);
	libc_lxstat = dlsym(RTLD_NEXT, "__lxstat");
	libc_close = dlsym(RTLD_NEXT, "close");
	libc_syscall = dlsym(RTLD_NEXT, "syscall");
}

int __lxstat(int ver, const char *path, struct stat *stat)
{
	int ret = libc_lxstat(ver, path, stat);

	if(ret == -1 && !strcmp(path, "/run/systemd/system/")) {
		stat->st_mode = S_IFDIR;
		ret = 0;
	}

	return ret;
}

int close(int fd)
{
	if(lock_fd != -1 && fd == lock_fd)
		return fcntl(fd, F_SETFD, FD_CLOEXEC);

	return libc_close(fd);
}

long syscall(long number, ...)
{
	unsigned long	clone_flags;
	va_list		ap;
	long		ret;

	/* XXX: we're targeting systemd-nspawn with this shim, its only syscall() use is __NR_clone */
	if(number != __NR_clone)
		return -1;

	va_start(ap, number);
	clone_flags = va_arg(ap, unsigned long);
	va_end(ap);

	ret = libc_syscall(number, clone_flags, NULL);

	if(ret > 0) {
		int fd;
		/* in parent; try record the container's pid */
		if((fd = open(PIDFILE_TMP, O_CREAT|O_WRONLY|O_SYNC, 0640)) != -1) {
			int	len;
			char	buf[20];

			if((len = snprintf(buf, sizeof(buf), "%li\n", ret)) != -1)
				if(write(fd, buf, len) == len)
					rename(PIDFILE_TMP, PIDFILE);

			libc_close(fd);
		}
	}

	return ret;
}
EOF

install -d "${ROOTDIR}/etc"
echo "rocket" > "${ROOTDIR}/etc/os-release"

# parent dir for the stage2 bind mounts
install -d "${ROOTDIR}/opt/stage2"

# dir for result code files
install -d "${ROOTDIR}/rkt/status"

# fin
mkdir "${BINDIR}"
tar cf "${BINDIR}/s1rootfs.tar" -C "${ROOTDIR}" .
OUTDIR=$(dirname "${OUTPUT}")
[ -d "$OUTDIR" ] || mkdir -p "${OUTDIR}"
go-bindata -o "${OUTPUT}" -prefix "${PWD}/${BINDIR}" -pkg=stage1_rootfs "${BINDIR}"
rm -Rf "${WORK}"

