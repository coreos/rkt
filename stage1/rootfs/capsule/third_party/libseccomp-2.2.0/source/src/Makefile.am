####
# Seccomp Library Source Files
#

#
# This library is free software; you can redistribute it and/or modify it
# under the terms of version 2.1 of the GNU Lesser General Public License
# as published by the Free Software Foundation.
#
# This library is distributed in the hope that it will be useful, but
# WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser
# General Public License for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with this library; if not, see <http://www.gnu.org/licenses>.
#

SUBDIRS = .
if ENABLE_PYTHON
SUBDIRS += python
endif

SOURCES_ARCH = \
	arch.c arch.h \
	arch-x86.h arch-x86.c arch-x86-syscalls.c \
	arch-x86_64.h arch-x86_64.c arch-x86_64-syscalls.c \
	arch-x32.h arch-x32.c arch-x32-syscalls.c \
	arch-arm.h arch-arm.c arch-arm-syscalls.c \
	arch-aarch64.h arch-aarch64.c arch-aarch64-syscalls.c \
	arch-mips.h arch-mips.c arch-mips-syscalls.c \
	arch-mips64.h arch-mips64.c arch-mips64-syscalls.c \
	arch-mips64n32.h arch-mips64n32.c arch-mips64n32-syscalls.c

SOURCES_GEN = \
	api.c system.h system.c \
	db.h db.c \
	hash.h hash.c \
	gen_pfc.h gen_pfc.c gen_bpf.h gen_bpf.c

EXTRA_DIST = arch-syscall-validate

TESTS = arch-syscall-check

check_PROGRAMS = arch-syscall-check arch-syscall-dump

lib_LTLIBRARIES = libseccomp.la

arch_syscall_dump_SOURCES = arch-syscall-dump.c ${SOURCES_ARCH}

arch_syscall_check_SOURCES = arch-syscall-check.c ${SOURCES_ARCH}

libseccomp_la_SOURCES = ${SOURCES_GEN} ${SOURCES_ARCH}
libseccomp_la_CFLAGS = ${AM_CFLAGS} ${CFLAGS} -fPIC -DPIC -fvisibility=hidden
libseccomp_la_LDFLAGS = ${AM_LDFLAGS} ${LDFLAGS} \
	-version-number ${VERSION_MAJOR}:${VERSION_MINOR}:${VERSION_MICRO}

check-build:
	${MAKE} ${AM_MAKEFLAGS} ${check_PROGRAMS}
