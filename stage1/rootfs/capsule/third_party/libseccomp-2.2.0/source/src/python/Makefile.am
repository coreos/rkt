####
# Seccomp Library Python Bindings
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

PYTHON = /usr/bin/env python

PY_DISTUTILS = \
	VERSION_RELEASE="@PACKAGE_VERSION@" \
	CPPFLAGS="-I\${top_srcdir}/include ${AM_CPPFLAGS} ${CPPFLAGS}" \
	CFLAGS="${AM_CFLAGS} ${CFLAGS}" \
	LDFLAGS="${AM_LDFLAGS} ${LDFLAGS}" \
	${PYTHON} ./setup.py

# support silent builds
PY_BUILD_0 = @echo "  PYTHON  " $@; ${PY_DISTUTILS} -q build
PY_BUILD_1 = ${PY_DISTUTILS} build
PY_BUILD_ = ${PY_BUILD_0}
PY_BUILD = ${PY_BUILD_@AM_V@}

PY_INSTALL = ${PY_DISTUTILS} install

EXTRA_DIST = libseccomp.pxd seccomp.pyx setup.py

all-local: build

build: ../libseccomp.la libseccomp.pxd seccomp.pyx setup.py
	${PY_BUILD} && touch build

install-exec-local: build
	${PY_INSTALL} --prefix=${DESTDIR}/${prefix}

clean-local:
	${RM} -rf seccomp.c build
