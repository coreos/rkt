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

def test(args):
    f = SyscallFilter(KILL)
    # syscalls referenced by number to make the test simpler
    f.add_rule_exactly(ALLOW, 1)
    i = 0
    while i < 100:
        f.add_rule_exactly(ALLOW, 1000,
                           Arg(0, EQ, i),
                           Arg(1, NE, 0),
                           Arg(2, LT, sys.maxsize))
        i += 1
    i = 100
    while i < 200:
        f.add_rule_exactly(ALLOW, i,
                           Arg(0, NE, 0))
        i += 1
    f.add_rule_exactly(ALLOW, 4)
    return f

args = util.get_opt()
ctx = test(args)
util.filter_output(args, ctx)

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;

