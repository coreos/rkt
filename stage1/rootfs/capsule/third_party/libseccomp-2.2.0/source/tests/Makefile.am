####
# Seccomp Library Tests
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

AM_LDFLAGS = -static
LDADD = util.la ../src/libseccomp.la

check_LTLIBRARIES = util.la
util_la_SOURCES = util.c util.h
util_la_LDFLAGS = -module

TESTS = regression

check_PROGRAMS = \
	01-sim-allow \
	02-sim-basic \
	03-sim-basic_chains \
	04-sim-multilevel_chains \
	05-sim-long_jumps \
	06-sim-actions \
	07-sim-db_bug_looping \
	08-sim-subtree_checks \
	09-sim-syscall_priority_pre \
	10-sim-syscall_priority_post \
	11-basic-basic_errors \
	12-sim-basic_masked_ops \
	13-basic-attrs \
	14-sim-reset \
	15-basic-resolver \
	16-sim-arch_basic \
	17-sim-arch_merge \
	18-sim-basic_whitelist \
	19-sim-missing_syscalls \
	20-live-basic_die \
	21-live-basic_allow \
	22-sim-basic_chains_array \
	23-sim-arch_all_le_basic \
	24-live-arg_allow \
	25-sim-multilevel_chains_adv \
	26-sim-arch_all_be_basic

EXTRA_DIST_TESTPYTHON = \
	util.py \
	01-sim-allow.py \
	02-sim-basic.py \
	03-sim-basic_chains.py \
	04-sim-multilevel_chains.py \
	05-sim-long_jumps.py \
	06-sim-actions.py \
	07-sim-db_bug_looping.py \
	08-sim-subtree_checks.py \
	09-sim-syscall_priority_pre.py \
	10-sim-syscall_priority_post.py \
	11-basic-basic_errors.py \
	12-sim-basic_masked_ops.py \
	13-basic-attrs.py \
	14-sim-reset.py \
	15-basic-resolver.py \
	16-sim-arch_basic.py \
	17-sim-arch_merge.py \
	18-sim-basic_whitelist.py \
	19-sim-missing_syscalls.py \
	20-live-basic_die.py \
	21-live-basic_allow.py \
	22-sim-basic_chains_array.py \
	23-sim-arch_all_le_basic.py \
	24-live-arg_allow.py \
	25-sim-multilevel_chains_adv.py \
	26-sim-arch_all_be_basic.py

EXTRA_DIST_TESTCFGS = \
	01-sim-allow.tests \
	02-sim-basic.tests \
	03-sim-basic_chains.tests \
	04-sim-multilevel_chains.tests \
	05-sim-long_jumps.tests \
	06-sim-actions.tests \
	07-sim-db_bug_looping.tests \
	08-sim-subtree_checks.tests \
	09-sim-syscall_priority_pre.tests \
	10-sim-syscall_priority_post.tests \
	11-basic-basic_errors.tests \
	12-sim-basic_masked_ops.tests \
	13-basic-attrs.tests \
	14-sim-reset.tests \
	15-basic-resolver.tests \
	16-sim-arch_basic.tests \
	17-sim-arch_merge.tests \
	18-sim-basic_whitelist.tests \
	19-sim-missing_syscalls.tests \
	20-live-basic_die.tests \
	21-live-basic_allow.tests \
	22-sim-basic_chains_array.tests \
	23-sim-arch_all_le_basic.tests \
	24-live-arg_allow.tests \
	25-sim-multilevel_chains_adv.tests \
	26-sim-arch_all_be_basic.tests

EXTRA_DIST_TESTSCRIPTS = regression testdiff testgen

EXTRA_DIST_TESTVALGRIND = valgrind_test.supp

EXTRA_DIST = \
	${EXTRA_DIST_TESTCFGS} \
	${EXTRA_DIST_TESTPYTHON} \
	${EXTRA_DIST_TESTSCRIPTS} \
	${EXTRA_DIST_TESTVALGRIND}

EXTRA_PROGRAMS = 00-test

check-build:
	${MAKE} ${AM_MAKEFLAGS} ${check_PROGRAMS}

clean-local:
	${RM} -f 00-test *.pyc
