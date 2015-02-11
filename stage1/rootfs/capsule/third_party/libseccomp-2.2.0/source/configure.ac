dnl ####
dnl # Seccomp Library
dnl #

dnl #
dnl # This library is free software; you can redistribute it and/or modify it
dnl # under the terms of version 2.1 of the GNU Lesser General Public License
dnl # as published by the Free Software Foundation.
dnl #
dnl # This library is distributed in the hope that it will be useful, but
dnl # WITHOUT ANY WARRANTY; without even the implied warranty of
dnl # MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser
dnl # General Public License for more details.
dnl #
dnl # You should have received a copy of the GNU Lesser General Public License
dnl # along with this library; if not, see <http://www.gnu.org/licenses>.
dnl #

dnl ####
dnl libseccomp defines
dnl ####
AC_INIT([libseccomp], [2.2.0])

dnl ####
dnl autoconf configuration
dnl ####
AC_CONFIG_AUX_DIR([build-aux])
AC_CONFIG_HEADERS([configure.h])
AC_CONFIG_MACRO_DIR([m4])

dnl ####
dnl automake configuration
dnl ####
dnl NOTE: Automake < 1.12 didn't have serial-tests and gives an error if it
dnl sees this, but for automake >= 1.13 serial-tests is required so we have to
dnl include it.  Solution is to test for the version of automake (by running
dnl an external command) and provide it if necessary.  Note we have to do this
dnl entirely using m4 macros since automake queries this macro by running
dnl 'autoconf --trace ...'.
m4_define([serial_tests], [
    m4_esyscmd([automake --version |
                head -1 |
                awk '{split ($NF,a,"."); if (a[1] == 1 && a[2] >= 12) { print "serial-tests" }}'
    ])
])
dnl # NOTE: do not [quote] this parameter
AM_INIT_AUTOMAKE(-Wall foreign subdir-objects tar-pax serial_tests)

dnl ####
dnl build tools
dnl ####
AC_PROG_CC
AM_PROG_CC_C_O
m4_ifdef([AM_PROG_AR], [AM_PROG_AR])

dnl ####
dnl libtool configuration
dnl ####
LT_INIT([shared pic-only])

dnl ####
dnl enable silent builds by default
dnl ####
m4_ifdef([AM_SILENT_RULES], [AM_SILENT_RULES([yes])])

dnl ####
dnl build flags
dnl ####
AM_CPPFLAGS="-I\${top_srcdir}/include"
AM_CFLAGS="-Wall"
AM_LDFLAGS="-Wl,-z -Wl,relro"
AC_SUBST([AM_CPPFLAGS])
AC_SUBST([AM_CFLAGS])
AC_SUBST([AM_LDFLAGS])

dnl ####
dnl check build system seccomp awareness
dnl ####
AC_CHECK_HEADERS_ONCE([linux/seccomp.h])

dnl ####
dnl version information
dnl ####
VERSION_MAJOR=$(echo ${VERSION} | cut -d'.' -f 1)
VERSION_MINOR=$(echo ${VERSION} | cut -d'.' -f 2)
VERSION_MICRO=$(echo ${VERSION} | cut -d'.' -f 3)
AC_SUBST([VERSION_MAJOR])
AC_SUBST([VERSION_MINOR])
AC_SUBST([VERSION_MICRO])

dnl ####
dnl function checks
dnl ####
dnl # NOTE: keep this disabled until we can test on a released 3.17 kernel
dnl AC_CHECK_FUNCS(seccomp)

dnl ####
dnl cython checks
dnl ####
AC_CHECK_PROG(have_cython, cython, "yes", "no")
AS_IF([test "$have_cython" = yes], [
	AS_ECHO("checking cython version... $(cython -V 2>&1 | cut -d' ' -f 3)")
	CYTHON_VER_MAJ=$(cython -V 2>&1 | cut -d' ' -f 3 | cut -d'.' -f 1);
	CYTHON_VER_MIN=$(cython -V 2>&1 | cut -d' ' -f 3 | cut -d'.' -f 2);
],[
	CYTHON_VER_MAJ=0
	CYTHON_VER_MIN=0
])

dnl ####
dnl python binding checks
dnl ####
AC_ARG_ENABLE([python],
	[AS_HELP_STRING([--enable-python],
	[build the python bindings, requires cython])])
AS_IF([test "$enable_python" = yes], [
	# cython version check
	AS_IF([test "$CYTHON_VER_MAJ" -eq 0 -a "$CYTHON_VER_MIN" -lt 16], [
		AC_MSG_ERROR([python bindings require cython 0.16 or higher])
	])
])
AM_CONDITIONAL([ENABLE_PYTHON], [test "$enable_python" = yes])
AC_DEFINE_UNQUOTED([ENABLE_PYTHON],
	[$(test "$enable_python" == yes && echo 1 || echo 0)],
	[Python bindings build flag.])

dnl ####
dnl coverity checks
dnl ####
AC_CHECK_PROG(have_coverity, cov-build, "yes", "no")
AM_CONDITIONAL(COVERITY, test "$have_coverity" = yes)

dnl ####
dnl version dependent files
dnl ####
AC_CONFIG_FILES([
	libseccomp.pc
	include/seccomp.h
])

dnl ####
dnl makefiles
dnl ####
AC_CONFIG_FILES([
	Makefile
	include/Makefile
	src/Makefile
	src/python/Makefile
	tools/Makefile
	tests/Makefile
	doc/Makefile
])

dnl ####
dnl done
dnl ####
AC_OUTPUT
