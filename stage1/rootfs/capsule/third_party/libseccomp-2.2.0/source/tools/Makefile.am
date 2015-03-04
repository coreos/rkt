####
# Seccomp Library Utility Tools
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

noinst_LTLIBRARIES = util.la
util_la_SOURCES = util.c util.h bpf.h
util_la_LDFLAGS = -module

bin_PROGRAMS = \
	scmp_sys_resolver
noinst_PROGRAMS = \
	scmp_arch_detect \
	scmp_bpf_disasm \
	scmp_bpf_sim

EXTRA_DIST = check-syntax scmp_app_inspector

scmp_bpf_disasm_SOURCES = scmp_bpf_disasm.c bpf.h util.h
scmp_bpf_sim_SOURCES = scmp_bpf_sim.c bpf.h util.h

scmp_sys_resolver_LDADD = ../src/libseccomp.la
scmp_sys_resolver_LDFLAGS = -static
scmp_arch_detect_LDADD = ../src/libseccomp.la
scmp_arch_detect_LDFLAGS = -static
scmp_bpf_disasm_LDADD = util.la
scmp_bpf_sim_LDADD = util.la
