#!/usr/bin/env python

#
# Seccomp Library test program
#
# Copyright (c) 2012 Red Hat <pmoore@redhat.com>
# Author: Paul Moore <pmoore@redhat.com>
#

#
# This library is free software; you can redistribute it and/or modify it
# under the terms of version 2.1 of the GNU Lesser General Public License as
# published by the Free Software Foundation.
#
# This library is distributed in the hope that it will be useful, but WITHOUT
# ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
# FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser General Public License
# for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with this library; if not, see <http://www.gnu.org/licenses>.
#

import argparse
import sys

import util

from seccomp import *

def test():
    f = SyscallFilter(KILL)
    # this differs from the native test as we don't support the syscall
    # resolution functions by themselves
    f.add_rule(ALLOW, "open")
    f.add_rule(ALLOW, "socket")
    try:
        f.add_rule(ALLOW, "INVALID")
    except RuntimeError:
        pass

    sys_num = resolve_syscall(Arch(), "open")
    sys_name = resolve_syscall(Arch(), sys_num)
    if (sys_name != "open"):
        raise RuntimeError("Test failure")
    sys_num = resolve_syscall(Arch(), "socket")
    sys_name = resolve_syscall(Arch(), sys_num)
    if (sys_name != "socket"):
        raise RuntimeError("Test failure")

test()

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
