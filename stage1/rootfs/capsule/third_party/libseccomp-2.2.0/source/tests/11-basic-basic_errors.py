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
    # this test differs from the native test for obvious reasons
    try:
        f = SyscallFilter(ALLOW + 1)
    except RuntimeError:
        pass

    f = SyscallFilter(ALLOW)
    try:
        f.reset(KILL + 1)
    except ValueError:
        pass

    f = SyscallFilter(ALLOW)
    try:
        f.syscall_priority(-10000, 1)
    except RuntimeError:
        pass

    f = SyscallFilter(ALLOW)
    try:
        f.add_rule(ALLOW, "read")
    except RuntimeError:
        pass
    try:
        f.add_rule(KILL - 1, "read")
    except RuntimeError:
        pass
    try:
        f.add_rule(KILL, "read",
                Arg(0, EQ, 0),
                Arg(1, EQ, 1),
                Arg(2, EQ, 2),
                Arg(3, EQ, 3),
                Arg(4, EQ, 4),
                Arg(5, EQ, 5),
                Arg(6, EQ, 6),
                Arg(7, EQ, 7))
    except RuntimeError:
        pass
    try:
        f.add_rule(KILL, -1001)
    except RuntimeError:
        pass

    f = SyscallFilter(ALLOW)
    f.remove_arch(Arch())
    f.add_arch(Arch("x86"))
    try:
        f.add_rule_exactly(KILL, "socket", Arg(0, EQ, 2))
    except RuntimeError:
        pass

test()

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
