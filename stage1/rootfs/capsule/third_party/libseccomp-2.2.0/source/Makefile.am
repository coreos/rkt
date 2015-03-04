####
# Seccomp Library
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

ACLOCAL_AMFLAGS = -I m4
SUBDIRS = include src tools tests doc

pkgconfdir = ${libdir}/pkgconfig
pkgconf_DATA = libseccomp.pc

EXTRA_DIST = CHANGELOG CREDITS LICENSE README SUBMITTING_PATCHES

# support silent builds
AM_MAKEFLAGS_0 = --quiet --no-print-directory
AM_MAKEFLAGS_1 =
AM_MAKEFLAGS_ = ${AM_MAKEFLAGS_0}
AM_MAKEFLAGS = ${AM_MAKEFLAGS_@AM_V@}

check-build: all
	${MAKE} ${AM_MAKEFLAGS} -C src check-build
	${MAKE} ${AM_MAKEFLAGS} -C tests check-build

check-syntax:
	@./tools/check-syntax

if COVERITY
coverity-build: clean
	cov-build --dir cov-int ${MAKE} ${AM_MAKEFLAGS} check-build
endif

if COVERITY
coverity-tarball: coverity-build
	@if git rev-parse HEAD &> /dev/null; then \
		rev_full=$$(git rev-parse HEAD); \
		rev=$$(echo $$rev_full | cut -c1-8); \
	else \
		rev_full=$$(date --iso-8601=date); \
		rev=$$rev_full; \
	fi; \
	tar czf libseccomp-coverity_$$rev.tar.gz cov-int; \
	echo " HEAD revision: $$rev_full"; \
	ls -l libseccomp-coverity_$$rev.tar.gz
endif

clean-local:
	${RM} -rf cov-int libseccomp-coverity_*.tar.gz
