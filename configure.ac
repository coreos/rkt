AC_PREREQ([2.69])
AC_INIT([rkt], [0.5.5+git], [https://github.com/coreos/rkt/issues])

# STAGE1 build settings

## STAGE1:type
AC_ARG_WITH(stage1,
        AS_HELP_STRING([--with-stage1=type],
                [type of stage1 build one of 'src', 'coreos', 'none' (default: 'coreos')]),
        [RKT_STAGE1_USR_FROM="$withval"],
        [RKT_STAGE1_USR_FROM="coreos"])

case "${RKT_STAGE1_USR_FROM}" in
	none)
        AC_MSG_RESULT([stage1 build disabled])
		;;
	coreos)
        AC_MSG_RESULT([stage1 will reuse coreos image])

        RKT_STAGE1_USR_LIB=/usr/lib64
        
        # check that some coreos-build specific binaries are here
        AC_CHECK_PROG([CURL],[curl],[curl])
        if [[ -z "$CURL" ]] ; then
            AC_MSG_ERROR([curl not found])
        fi

        AC_CHECK_PROG([GPG],[gpg],[gpg])
        if [[ -z "$GPG" ]] ; then
            AC_MSG_ERROR([gpg not found])
        fi

        AC_CHECK_PROG([MKTEMP],[mktemp],[mktemp])
        if [[ -z "$MKTEMP" ]] ; then
            AC_MSG_ERROR([mktemp not found])
        fi

        AC_CHECK_PROG([MD5SUM],[md5sum],[md5sum])
        if [[ -z "$MD5SUM" ]] ; then
            AC_MSG_ERROR([md5sum not found])
        fi

        AC_CHECK_PROG([CPIO],[cpio],[cpio])
        if [[ -z "$CPIO" ]] ; then
            AC_MSG_ERROR([cpio not found])
        fi

        AC_CHECK_PROG([GZIP],[gzip],[gzip])
        if [[ -z "$GZIP" ]] ; then
            AC_MSG_ERROR([gzip not found])
        fi

        AC_CHECK_PROG([UNSQUASHFS],[unsquashfs],[unsquashfs])
        if [[ -z "$UNSQUASHFS" ]] ; then
            AC_MSG_ERROR([unsquashfs not found])
        fi

        AC_CHECK_PROG([SORT],[sort],[sort])
        if [[ -z "$SORT" ]] ; then
            AC_MSG_ERROR([sort not found])
        fi
        
		;;
	src)
        AC_MSG_RESULT([stage1 will build systemd from source])
        RKT_STAGE1_USR_LIB=/usr/lib

        # these are needed for systemd build
                
        AC_CHECK_PROG([INTLTOOLIZE],[intltoolize],[intltoolize])
        if [[ -z "$INTLTOOLIZE" ]] ; then
            AC_MSG_ERROR([intltoolize not installed])
        fi

        AC_CHECK_PROG([LIBTOOLIZE],[libtoolize],[libtoolize])
        if [[ -z "$LIBTOOLIZE" ]] ; then
            AC_MSG_ERROR([libtoolize not installed])
        fi
		;;
	*)
		AC_MSG_ERROR([stage1 build type '${RKT_STAGE1_USR_FROM}' unsupported])
esac

AC_SUBST(RKT_STAGE1_USR_FROM)
AC_SUBST(RKT_STAGE1_USR_LIB)

## STAGE1: Systemd git path
AC_ARG_WITH(stage1-systemd-src,
        AS_HELP_STRING([--with-stage1-systemd-src=git-path],
                [address to git repository of systemd, used in 'src' build mode (default: git://anongit.freedesktop.org/systemd/systemd)]),
        [RKT_STAGE1_SYSTEMD_SRC="$withval"],
        [RKT_STAGE1_SYSTEMD_SRC="git://anongit.freedesktop.org/systemd/systemd"])

AC_SUBST(RKT_STAGE1_SYSTEMD_SRC)

## STAGE1: Systemd version
AC_ARG_WITH(stage1-systemd-version,
        AS_HELP_STRING([--with-stage1-systemd-version=version],
                [systemd version to build (default: v219)]),
        [RKT_STAGE1_SYSTEMD_VER="$withval"],
        [RKT_STAGE1_SYSTEMD_VER="v219"])

AC_SUBST(RKT_STAGE1_SYSTEMD_VER)

## STAGE1: linker-defined custom STAGE1 image path, default is unset
AC_ARG_WITH(stage1-image-path,
        AS_HELP_STRING([--with-stage1-image-path],
                [custom stage1 image path (default: "")]),
        [RKT_STAGE1_IMAGE="$withval"],
        [RKT_STAGE1_IMAGE=""])

RKT_STAGE1_IMAGE_FLAGS=
# if stage1 image variable is set, add a linker flag to rkt defining the variable
if [[ -n "$RKT_STAGE1_IMAGE" ]] ; then
   RKT_STAGE1_IMAGE_FLAGS="-ldflags \"-X main.defaultStage1Image '${RKT_STAGE1_IMAGE}'\""
fi

AC_SUBST(RKT_STAGE1_IMAGE_FLAGS)

## STAGE1 D-Bus parameters
AC_ARG_ENABLE(stage1-dbus,
        AS_HELP_STRING([--enable-stage1-dbus=no],
                [enable D-Bus in stage1 (default: no)]),
        [RKT_STAGE1_DBUS="yes"],
        [RKT_STAGE1_DBUS="no"])

AC_SUBST(RKT_STAGE1_DBUS)

AC_ARG_WITH(stage1-dbus-src,
        AS_HELP_STRING([--with-stage1-dbus-src=git-path],
                [address to git repository of D-Bus, used in 'src' build mode (default: git://anongit.freedesktop.org/dbus/dbus)]),
        [RKT_STAGE1_DBUS_SRC="$withval"],
        [RKT_STAGE1_DBUS_SRC="git://anongit.freedesktop.org/dbus/dbus"])
 
AC_SUBST(RKT_STAGE1_DBUS_SRC)

AC_ARG_WITH(stage1-dbus-version,
        AS_HELP_STRING([--with-stage1-dbus-version=version],
                [D-Bus version to build (default: dbus-1.8.16)]),
        [RKT_STAGE1_DBUS_VER="$withval"],
        [RKT_STAGE1_DBUS_VER="dbus-1.8.16"])

AC_SUBST(RKT_STAGE1_DBUS_VER)

if [[ "$RKT_STAGE1_DBUS" = "yes" ]] ; then

AC_MSG_RESULT([stage1 will build D-Bus])

AC_SEARCH_LIBS([_nss_files_gethostbyname_r], [nss_files], [],
               [AC_MSG_ERROR([libnss files is needed to have D-Bus working properly. Install libc package])])
AC_SUBST(NSS_FILES_LIB, [$ac_cv_search__nss_files_gethostbyname_r])


AC_SEARCH_LIBS([sd_listen_fds], [systemd], [],
               [AC_MSG_ERROR([systemd dev library needed to build D-Bus from source. Install libc package])])


fi


# Checks for programs.
AC_PROG_CC
AC_PROG_INSTALL

AC_CHECK_PROG([GIT],[git],[git])
if [[ -z "${GIT}" ]] ; then
    AC_MSG_ERROR([git not installed])
fi

AC_CHECK_PROG([GOBINARY],[go],[go])
if [[ -z "${GOBINARY}" ]] ; then
    AC_MSG_ERROR([Go language binary not found.])
fi

AC_CHECK_PROG([GOFMTBINARY],[gofmt],[gofmt])
if [[ -z "${GOFMTBINARY}" ]] ; then
    AC_MSG_ERROR([gofmt binary not found.])
fi

AC_CHECK_PROG([BASH],[bash],[bash])
if [[ -z "${BASH}" ]] ; then
    AC_MSG_ERROR([bash not found])
fi

AC_CONFIG_FILES([Makefile])
AC_OUTPUT
