AC_PREREQ([2.69])
AC_INIT([rkt], [0.5.6+git], [https://github.com/coreos/rkt/issues])

AC_CANONICAL_HOST

AC_DEFUN([RKT_REQ_PROG],
         [AS_VAR_IF([$1], [],
                    [AC_CHECK_PROG($@)
                     AS_VAR_IF([$1], [$3], [], AC_MSG_ERROR([$2 not found]))],
                    [])])

AC_DEFUN([RKT_REQ_ABS_PROG],
         [AS_VAR_IF([$1], [],
                    [AC_PATH_PROG($@)
		     AS_IF([test "x$]$1[" = 'x'], [AC_MSG_ERROR([$2 not found])])],
                    [])])

# STAGE1 build settings
# We assume having bash in lots of places, so check it in the
# beginning.
RKT_REQ_ABS_PROG([BASH_SHELL], [bash])
RKT_REQ_ABS_PROG([ABS_GO], [go])

## STAGE1:type
m4_define([DEFAULT_FLAVOR], [coreos])
AC_ARG_WITH(stage1,
            AS_HELP_STRING([--with-stage1=type],
                           [type of stage1 build one of 'src', 'coreos', 'host', 'none' (default: 'DEFAULT_FLAVOR')]),
            [RKT_STAGE1_USR_FROM="${withval}"],
            [RKT_STAGE1_USR_FROM="coreos"])

AS_CASE([${RKT_STAGE1_USR_FROM}],
        [none],
                [],
        [coreos],
                [# check that some coreos-build specific binaries are here
                 RKT_REQ_PROG([WGET],[wget],[wget])
                 RKT_REQ_PROG([GPG],[gpg],[gpg])
                 RKT_REQ_PROG([MKTEMP],[mktemp],[mktemp])
                 RKT_REQ_PROG([MD5SUM],[md5sum],[md5sum])
                 RKT_REQ_PROG([CPIO],[cpio],[cpio])
                 RKT_REQ_PROG([GZIP],[gzip],[gzip])
                 RKT_REQ_PROG([UNSQUASHFS],[unsquashfs],[unsquashfs])
                 RKT_REQ_PROG([SORT],[sort],[sort])],
        [src],
                [# these are needed for systemd build
                 RKT_REQ_PROG([INTLTOOLIZE],[intltoolize],[intltoolize])
                 RKT_REQ_PROG([LIBTOOLIZE],[libtoolize],[libtoolize])],
        [host],
                [],
        [AC_MSG_ERROR([stage1 build type '${RKT_STAGE1_USR_FROM}' unsupported])])

AC_SUBST(RKT_STAGE1_USR_FROM)

## STAGE1: Systemd git path for src flavor
m4_define([DEFAULT_STAGE1_SYSTEMD_SRC], [https://github.com/systemd/systemd.git])
AC_ARG_WITH(stage1-systemd-src,
            AS_HELP_STRING([--with-stage1-systemd-src=git-path],
                           [address to git repository of systemd, used in 'src' build mode (default: 'DEFAULT_STAGE1_SYSTEMD_SRC')]),
            [RKT_STAGE1_SYSTEMD_SRC="${withval}"],
            [RKT_STAGE1_SYSTEMD_SRC='auto'])

AS_VAR_IF([RKT_STAGE1_SYSTEMD_SRC], [auto],
          [],
          [AS_VAR_IF([RKT_STAGE1_USR_FROM], [src],
                     [],
                     [AC_MSG_WARN(--with-stage1-systemd-src is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor)])])


AS_VAR_IF([RKT_STAGE1_SYSTEMD_SRC], [auto],
          [AS_VAR_IF([RKT_STAGE1_USR_FROM], [src],
                     [RKT_STAGE1_SYSTEMD_SRC=DEFAULT_STAGE1_SYSTEMD_SRC],
                     [])],
          [])

AC_SUBST(RKT_STAGE1_SYSTEMD_SRC)

## STAGE1: Systemd version
m4_define([DEFAULT_STAGE1_SYSTEMD_VER], [v220])
AC_ARG_WITH(stage1-systemd-version,
            AS_HELP_STRING([--with-stage1-systemd-version=version],
                           [systemd version to build (default: 'DEFAULT_STAGE1_SYSTEMD_VER')]),
            [RKT_STAGE1_SYSTEMD_VER="${withval}"],
            [RKT_STAGE1_SYSTEMD_VER='auto'])

AS_VAR_IF([RKT_STAGE1_SYSTEMD_VER], [auto],
          [],
          [AS_VAR_IF([RKT_STAGE1_USR_FROM], [src],
                     [],
                     [AC_MSG_WARN([--with-stage1-systemd-version is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])])])


AS_VAR_IF([RKT_STAGE1_SYSTEMD_VER], [auto],
          [AS_VAR_IF([RKT_STAGE1_USR_FROM], [src],
                     [RKT_STAGE1_SYSTEMD_VER=DEFAULT_STAGE1_SYSTEMD_VER],
                     [])],
          [])

AS_VAR_IF([RKT_STAGE1_SYSTEMD_VER], [HEAD],
          [AS_VAR_IF([RKT_STAGE1_USR_FROM], [src],
                     [AC_MSG_WARN(['HEAD' is not a systemd version, setting it to 'master' instead])]
                     [RKT_STAGE1_SYSTEMD_VER=master],
                     [])],
          [])

AC_SUBST(RKT_STAGE1_SYSTEMD_VER)

## STAGE1: linker-defined custom STAGE1 image path, default is unset
m4_define([DEFAULT_STAGE1_IMAGE], [])
AC_ARG_WITH(stage1-image-path,
            AS_HELP_STRING([--with-stage1-image-path],
                           [custom stage1 image path (default: 'DEFAULT_STAGE1_IMAGE')]),
            [RKT_STAGE1_IMAGE="${withval}"],
            [RKT_STAGE1_IMAGE=""])

RKT_STAGE1_IMAGE_FLAGS=
# if stage1 image variable is set, add a linker flag to rkt defining the variable
AS_VAR_IF([RKT_STAGE1_IMAGE], [],
          [],
          [RKT_STAGE1_IMAGE_FLAGS="-ldflags \"-X main.defaultStage1Image '${RKT_STAGE1_IMAGE}'\""])

AC_SUBST(RKT_STAGE1_IMAGE_FLAGS)

AC_ARG_ENABLE([functional-tests],
              AS_HELP_STRING([--enable-functional-tests=no],
                             [enable functional tests on make check (linux only, uses sudo)]),
              [RKT_RUN_FUNCTIONAL_TESTS="${enableval}"],
              [RKT_RUN_FUNCTIONAL_TESTS="no"])

AS_VAR_IF([RKT_RUN_FUNCTIONAL_TESTS], [yes],
          [AC_MSG_CHECKING([whether functional tests can be run])]
          [AS_CASE([${host}],
                   [*-*-linux*],
                           [AS_VAR_IF([RKT_STAGE1_USR_FROM], [host],
                                      dnl checking if we are running systemd as init, -.mount is a mount unit for /.
                                      [AS_IF([systemctl 2>/dev/null | grep --silent -e '-\.mount'],
                                             [sdv=`systemctl --version | head -1 | sed -e 's/^systemd \(@<:@0-9@:>@*\)$/\1/'`]
                                             [AS_IF([test ${sdv} -lt 220],
                                                    [AC_MSG_RESULT([no])]
                                                    [AC_MSG_ERROR([Cannot run functional tests with ${RKT_STAGE1_USR_FROM} stage1 flavor and with version of systemd on host lower than 220 (currently running ${sdv})])])])])],
                   [AC_MSG_RESULT([no])]
                   [AC_MSG_ERROR([functional tests are supported only under linux])])])

AC_SUBST(RKT_RUN_FUNCTIONAL_TESTS)

# Checks for programs.
AC_PROG_CC
AC_PROG_INSTALL

RKT_REQ_PROG([GIT],[git],[git])
RKT_REQ_PROG([GOBINARY],[go],[go])
RKT_REQ_PROG([GOFMTBINARY],[gofmt],[gofmt])


# Checks for perl and its modules
RKT_REQ_PROG([PERL], [perl], [perl])
AS_VAR_IF([PERL], [perl],
	[for module in Capture::Tiny; do
		have_perl_module=no
		AC_MSG_CHECKING([for Perl $module module])
		AS_IF([perl -e "use $module" 2>/dev/null],
			[have_perl_module=yes])
		AC_MSG_RESULT([$have_perl_module])
		AS_IF([test "$have_perl_module" != "yes"],
			AC_MSG_ERROR([*** perl module '$module' not found]))
	done]
)


AC_CONFIG_FILES([makelib/variables.mk])
AC_OUTPUT
AC_MSG_RESULT([
        AC_PACKAGE_NAME AC_PACKAGE_VERSION

        stage1 flavor:          '${RKT_STAGE1_USR_FROM}'
        stage1 image path:      '${RKT_STAGE1_IMAGE}'
        linker flags:           '${RKT_STAGE1_IMAGE_FLAGS}'
        functional tests:       '${RKT_RUN_FUNCTIONAL_TESTS}'])

AS_CASE([${RKT_STAGE1_USR_FROM}],
        [src],
                [AC_MSG_RESULT([
        systemd git repo:       '${RKT_STAGE1_SYSTEMD_SRC}'
        systemd version:        '${RKT_STAGE1_SYSTEMD_VER}'])],
        [])
