AC_PREREQ([2.68])
AC_INIT([rkt], [0.8.0+git], [https://github.com/coreos/rkt/issues])

dnl if version ends with +git, append a short git-hash.
AS_IF([test `expr match 'AC_PACKAGE_VERSION' '.*+git$'` -gt 0],
      dnl version has +git suffix, ignore errors (not a git repo)
      [RKT_VERSION="AC_PACKAGE_VERSION`git rev-parse --short HEAD 2>/dev/null``git diff-index --quiet HEAD 2>/dev/null || echo -dirty`"],
      dnl version has no +git suffix
      [RKT_VERSION="AC_PACKAGE_VERSION"])

RKT_VERSION_LDFLAGS="-X github.com/coreos/rkt/version.Version '${RKT_VERSION}'"
AC_SUBST(RKT_VERSION)
AC_SUBST(RKT_VERSION_LDFLAGS)

AC_CANONICAL_HOST
AC_CANONICAL_BUILD

AC_DEFUN([RKT_REQ_PROG],
         [AS_VAR_IF([$1], [],
                    [AC_CHECK_PROG($@)
                     AS_VAR_IF([$1], [$3], [], AC_MSG_ERROR([*** $2 not found]))],
                    [])])

AC_DEFUN([RKT_REQ_ABS_PROG],
         [AS_VAR_IF([$1], [],
                    [AC_PATH_PROG($@)
                     AS_IF([test "x$]$1[" = 'x'], [AC_MSG_ERROR([*** $2 not found])])],
                    [])])

# STAGE1 build settings
# We assume having bash in lots of places, so check it in the
# beginning.
RKT_REQ_ABS_PROG([BASH_SHELL], [bash])
RKT_REQ_ABS_PROG([ABS_GO], [go])

## STAGE1:type
m4_define([DEFAULT_FLAVOR], [coreos])
AC_ARG_WITH([stage1],
            AS_HELP_STRING([--with-stage1=type],
                           [type of stage1 build one of 'src', 'coreos', 'host', 'none', 'kvm' (default: 'DEFAULT_FLAVOR')]),
            [RKT_STAGE1_USR_FROM="${withval}"],
            [RKT_STAGE1_USR_FROM="DEFAULT_FLAVOR"])

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
        [kvm],
                [# just a warrning because of nature of this flavor
                 AC_MSG_WARN([* kvm is an experimental stage1 implementation, some features are missing])],
        [AC_MSG_ERROR([*** stage1 build type '${RKT_STAGE1_USR_FROM}' unsupported])])

AC_SUBST(RKT_STAGE1_USR_FROM)

m4_define(DEFAULT_STAGE1_NAME,coreos.com/rkt/stage1)
AC_ARG_WITH(stage1-default-name,
            AS_HELP_STRING([--with-stage1-default-name=stage1_name],
                           [default name of stage1, default: DEFAULT_STAGE1_NAME]),
            [RKT_STAGE1_DEFAULT_NAME="${withval}"],
            [RKT_STAGE1_DEFAULT_NAME=auto])

AC_ARG_WITH(stage1-default-version,
            AS_HELP_STRING([--with-stage1-default-version=stage1_version],
                           [default version of stage1, default: same as rkt version]),
            [RKT_STAGE1_DEFAULT_VERSION="${withval}"],
            [RKT_STAGE1_DEFAULT_VERSION=auto])

dnl warn if we specified above flags for other flavors than none
AS_VAR_IF([RKT_STAGE1_USR_FROM], [none],
          dnl none flavor
          [],
          dnl not none flavor
          [AS_VAR_IF([RKT_STAGE1_DEFAULT_NAME], [auto],
                     dnl default
                     [],
                     dnl not default
                     [AC_MSG_WARN([* --with-stage1-default-name is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])
                      RKT_STAGE1_DEFAULT_NAME=auto])
           AS_VAR_IF([RKT_STAGE1_DEFAULT_VERSION], [auto],
                     dnl default
                     [],
                     dnl not default
                     [AC_MSG_WARN([* --with-stage1-default-version is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])
                      RKT_STAGE1_DEFAULT_VERSION=auto])])

AS_VAR_IF([RKT_STAGE1_DEFAULT_NAME], [auto],
          [RKT_STAGE1_DEFAULT_NAME=DEFAULT_STAGE1_NAME])
AS_VAR_IF([RKT_STAGE1_DEFAULT_VERSION], [auto],
          [RKT_STAGE1_DEFAULT_VERSION=${RKT_VERSION}])

RKT_STAGE1_DEFAULT_ACI_LDFLAGS="-X main.defaultStage1Name '${RKT_STAGE1_DEFAULT_NAME}' -X main.defaultStage1Version '${RKT_STAGE1_DEFAULT_VERSION}'"

AC_SUBST(RKT_STAGE1_DEFAULT_NAME)
AC_SUBST(RKT_STAGE1_DEFAULT_VERSION)
AC_SUBST(RKT_STAGE1_DEFAULT_ACI_LDFLAGS)

## STAGE1: Systemd git path and version for src flavor

m4_define([DEFAULT_STAGE1_SYSTEMD_SRC], [https://github.com/systemd/systemd.git])
AC_ARG_WITH([stage1-systemd-src],
            AS_HELP_STRING([--with-stage1-systemd-src=git-path],
                           [address to git repository of systemd, used in 'src' stage1 flavor (default: 'DEFAULT_STAGE1_SYSTEMD_SRC')]),
            [RKT_STAGE1_SYSTEMD_SRC="${withval}"],
            [RKT_STAGE1_SYSTEMD_SRC='auto'])

m4_define([DEFAULT_STAGE1_SYSTEMD_VER], [v222])
AC_ARG_WITH([stage1-systemd-version],
            AS_HELP_STRING([--with-stage1-systemd-version=version],
                           [systemd version to build, used in 'src' stage1 flavor (default: 'DEFAULT_STAGE1_SYSTEMD_VER', should be in format 'v<number>', like v222)]),
            [RKT_STAGE1_SYSTEMD_VER="${withval}"],
            [RKT_STAGE1_SYSTEMD_VER='auto'])

AS_CASE([$RKT_STAGE1_USR_FROM],
        [src],
                [AS_VAR_IF([RKT_STAGE1_SYSTEMD_SRC], [auto],
                           dnl systemd source not specified, use default
                           [RKT_STAGE1_SYSTEMD_SRC=DEFAULT_STAGE1_SYSTEMD_SRC],
                           dnl systemd source specified, use it
                           [])
                 AS_VAR_IF([RKT_STAGE1_SYSTEMD_VER], [auto],
                           dnl systemd version not specified, use default
                           [RKT_STAGE1_SYSTEMD_VER=DEFAULT_STAGE1_SYSTEMD_VER],
                           dnl systemd version specified, use it
                           [])],
        dnl other flavors
        [AS_VAR_IF([RKT_STAGE1_SYSTEMD_SRC], [auto],
                   dnl systemd source not specified, alright, zero the value just in case
                   [RKT_STAGE1_SYSTEMD_SRC=''],
                   dnl systemd source specified, warn about it being ignored
                   [AC_MSG_WARN([* --with-stage1-systemd-src is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])])
         AS_VAR_IF([RKT_STAGE1_SYSTEMD_VER], [auto],
                   dnl systemd version not specified, alright, zero the value just in case
                   [RKT_STAGE1_SYSTEMD_VER=''],
                   dnl systemd version specified, warn about it being ignored
                   [AC_MSG_WARN([* --with-stage1-systemd-version is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])])])

dnl warn if version is HEAD, just use master
AS_VAR_IF([RKT_STAGE1_SYSTEMD_VER], [HEAD],
          [AC_MSG_WARN([* 'HEAD' is not a systemd version, setting it to 'master' instead])
           RKT_STAGE1_SYSTEMD_VER='master'],
          [])

AC_SUBST(RKT_STAGE1_SYSTEMD_SRC)
AC_SUBST(RKT_STAGE1_SYSTEMD_VER)

## STAGE1: linker-defined custom STAGE1 image path, default is unset
m4_define([DEFAULT_STAGE1_IMAGE], [])
AC_ARG_WITH([stage1-image-path],
            AS_HELP_STRING([--with-stage1-image-path],
                           [custom stage1 image path (default: 'DEFAULT_STAGE1_IMAGE')]),
            [RKT_STAGE1_IMAGE="${withval}"],
            [RKT_STAGE1_IMAGE=""])

RKT_STAGE1_IMAGE_LDFLAGS=
# if stage1 image variable is set, add a linker flag to rkt defining the variable
AS_VAR_IF([RKT_STAGE1_IMAGE], [],
          [],
          [RKT_STAGE1_IMAGE_LDFLAGS="-X main.defaultStage1Image '${RKT_STAGE1_IMAGE}'"])

AC_SUBST(RKT_STAGE1_IMAGE_LDFLAGS)

## STAGE1: path to coreos pxe and its systemd version

m4_define([DEFAULT_LOCAL_COREOS_PXE_IMAGE_PATH], [])
AC_ARG_WITH([coreos-local-pxe-image-path],
            AS_HELP_STRING([--with-coreos-local-pxe-image-path],
                           [path to local CoreOS PXE image, used in 'coreos' and 'kvm' stage1 flavors (default: 'DEFAULT_LOCAL_COREOS_PXE_IMAGE_PATH') (the GPG signature of this file will not be checked!)]),
            [RKT_LOCAL_COREOS_PXE_IMAGE_PATH="${withval}"],
            [RKT_LOCAL_COREOS_PXE_IMAGE_PATH=""])

AC_ARG_WITH([coreos-local-pxe-image-systemd-version],
            AS_HELP_STRING([--with-coreos-local-pxe-image-systemd-version],
                           [version of systemd in local CoreOS PXE image, used in 'coreos' and 'kvm' stage1 flavors (should be in format 'v<number>', like v222)]),
            [RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER="${withval}"],
            [RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER=])

AS_CASE([$RKT_STAGE1_USR_FROM],
        dnl for kvm and coreos check if both pxe image path and
        dnl systemd version are specified or neither
        [kvm|coreos],
                [AS_VAR_IF([RKT_LOCAL_COREOS_PXE_IMAGE_PATH], [],
                           dnl no local coreos pxe image path specified
                           [AS_VAR_IF([RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER],[],
                                      dnl no version specified, alright, keep quiet
                                      [],
                                      dnl version specified, warn about it being ignored
                                      [AC_MSG_WARN([* --with-coreos-local-pxe-image-systemd-version is ignored, because --with-coreos-local-pxe-image-path was not specified])
                                       RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER=''])],
                           dnl local coreos pxe image path specified
                           [AS_VAR_IF([RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER],[],
                                      dnl no version specified, bail out, version is required
                                      [AC_MSG_ERROR([*** No systemd version for local CoreOS PXE image specified, use --with-coreos-local-pxe-image-systemd-version])],
                                      dnl version specified, alright, keep quiet
                                      [])])],
        dnl for other flavors just warn if either is specified
        [AS_VAR_IF([RKT_LOCAL_COREOS_PXE_IMAGE_PATH], [],
                   dnl empty value, alright
                   [],
                   dnl not empty, warn about it, zero it just in case
                   [AC_MSG_WARN([* --with-coreos-local-pxe-image-path is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])
                    RKT_LOCAL_COREOS_PXE_IMAGE_PATH=''])
         AS_VAR_IF([RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER], [],
                   dnl empty value, alright
                   [],
                   dnl not empty, warn about it, zero it just in case
                   [AC_MSG_WARN([* --with-coreos-local-pxe-image-systemd-version is ignored by ${RKT_STAGE1_USR_FROM} stage1 flavor])
                    RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER=''])])

AC_SUBST(RKT_LOCAL_COREOS_PXE_IMAGE_PATH)
AC_SUBST(RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER)

## Functional tests

AC_ARG_ENABLE([functional-tests],
              AS_HELP_STRING([--enable-functional-tests],
                             [enable functional tests on make check (linux only, uses sudo, default: no, use auto to enable if possible, for host stage1 flavor systemd version 220 or higher on host is required)]),
              [RKT_RUN_FUNCTIONAL_TESTS="${enableval}"],
              [RKT_RUN_FUNCTIONAL_TESTS="no"])

AS_IF([test "x${RKT_RUN_FUNCTIONAL_TESTS}" = 'xyes' -o "x${RKT_RUN_FUNCTIONAL_TESTS}" = 'xauto'],
      [AC_MSG_CHECKING([whether functional tests can be run])
       rkt_functional_tests_msg=''
       AS_CASE([${host}],
               [*-*-linux*],
                       [AS_VAR_IF([RKT_STAGE1_USR_FROM], [host],
                                  dnl checking if we are running systemd as init, -.mount is a mount unit for /.
                                  [AS_IF([systemctl 2>/dev/null | grep --silent -e '-\.mount'],
                                         dnl systemd runs as init
                                         [sdv=`systemctl --version | head -1 | sed -e 's/^systemd \(@<:@0-9@:>@*\)$/\1/'`
                                          AS_IF([test ${sdv} -lt 220],
                                                [rkt_functional_tests_msg="Cannot run functional tests with ${RKT_STAGE1_USR_FROM} stage1 flavor - systemd version on host is lower than 220 (currently running ${sdv})"])],
                                         dnl systemd is not an init
                                         [rkt_functional_tests_msg="Cannot run functional tests with ${RKT_STAGE1_USR_FROM} stage1 flavor - no systemd on host"])])],
               dnl non-linux host
               [rkt_functional_tests_msg="functional tests are supported only under linux"])
      AS_VAR_IF([rkt_functional_tests_msg], [],
                 dnl no message, tests can be run
                 [RKT_RUN_FUNCTIONAL_TESTS=yes
                  AC_MSG_RESULT([yes])],
                 dnl message available, tests will not be run
                 [AC_MSG_RESULT([no])
                  AS_VAR_IF([RKT_RUN_FUNCTIONAL_TESTS], [auto],
                            dnl functional tests are optional, so only print an info
                            [RKT_RUN_FUNCTIONAL_TESTS=no
                             AC_MSG_NOTICE([${rkt_functional_tests_msg}])],
                            dnl functional tests are required, so bail out
                            [AC_MSG_ERROR([*** ${rkt_functional_tests_msg}])])])])

AC_SUBST(RKT_RUN_FUNCTIONAL_TESTS)

AC_ARG_ENABLE([shared],
              AS_HELP_STRING([--enable-shared],
                             [enable linking with shared library, default: no]),
              [RKT_LINK_SHARED_LIBS="${enableval}"],
              [RKT_LINK_SHARED_LIBS="no"])

AS_VAR_IF([RKT_LINK_SHARED_LIBS], [yes],
          dnl yes flavor
          [],
          dnl no flavor
          [RKT_STATIC_LIBS_LDFLAGS="-extldflags \"-static\""])

AC_SUBST(RKT_STATIC_LIBS_LDFLAGS)

# cross_compiling is a standard autoconf variable.  See the autoconf manual for
# details.
AS_VAR_IF([cross_compiling], [no],
          dnl no crosscompilation
          [GOARCH_FOR_BUILD=${GOARCH_FOR_BUILD:-${GOARCH}}],
          dnl we do crosscompilation
          [AC_MSG_CHECKING([build golang arch])
           AS_VAR_IF([GOARCH_FOR_BUILD], [],
                     [AS_CASE([${build_cpu}],
                              [x86_64],
                                      [GOARCH_FOR_BUILD="amd64"],
                              [aarch64],
                                      [GOARCH_FOR_BUILD="arm64"],
                              [powerpc],
                                      [GOARCH_FOR_BUILD="ppc64"],
                              [AC_MSG_RESULT([unknown]); AC_MSG_ERROR([unknown build cpu: ${build_cpu}.  Set GOARCH_FOR_BUILD variable.])])
                      AC_MSG_RESULT([${GOARCH_FOR_BUILD}])],
                     [AC_MSG_RESULT([user supplied ${GOARCH_FOR_BUILD}])])])

AC_SUBST(GOARCH_FOR_BUILD)

# Checks for programs.
AC_PROG_CC
AC_PROG_INSTALL

RKT_REQ_PROG([FILE],[file],[file])
RKT_REQ_PROG([GIT],[git],[git])
RKT_REQ_PROG([GOBINARY],[go],[go])
RKT_REQ_PROG([GOFMTBINARY],[gofmt],[gofmt])

AC_CONFIG_FILES([makelib/variables.mk
                 stage1/aci/aci-manifest.test:stage1/aci/aci-manifest.in])
AC_CONFIG_COMMANDS([stage1/aci/aci-manifest], [if cmp --silent stage1/aci/aci-manifest.test stage1/aci/aci-manifest; then :; else cp stage1/aci/aci-manifest.test stage1/aci/aci-manifest; fi;])
AC_OUTPUT
AC_MSG_RESULT([
        AC_PACKAGE_NAME ${RKT_VERSION}

        general build parameters

        stage1 flavor:                          '${RKT_STAGE1_USR_FROM}'
        stage1 image path:                      '${RKT_STAGE1_IMAGE}'
        functional tests:                       '${RKT_RUN_FUNCTIONAL_TESTS}'])

AC_MSG_RESULT([
        ${RKT_STAGE1_USR_FROM} flavor specific build parameters])
AS_CASE([${RKT_STAGE1_USR_FROM}],
        [src],
                [AC_MSG_RESULT([
        systemd git repo:                       '${RKT_STAGE1_SYSTEMD_SRC}'
        systemd version:                        '${RKT_STAGE1_SYSTEMD_VER}'])],
        [coreos|kvm],
                [AC_MSG_RESULT([
        local CoreOS PXE image path:            '${RKT_LOCAL_COREOS_PXE_IMAGE_PATH}'
        local CoreOS PXE image systemd version: '${RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER}'])],
        [none],
                [AC_MSG_RESULT([
        stage1 name:                            '${RKT_STAGE1_DEFAULT_NAME}'
        stage1 version:                         '${RKT_STAGE1_DEFAULT_VERSION}'])],
        [AC_MSG_RESULT([
        <none>])])
