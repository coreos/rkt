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
    # the syscall and argument numbers are all fake to make the test simpler
    f.add_rule_exactly(ALLOW, 1000,
                       Arg(0, EQ, 0),
                       Arg(1, EQ, 1),
                       Arg(2, EQ, 2))
    f.add_rule_exactly(ALLOW, 1000,
                       Arg(0, EQ, 0),
                       Arg(1, MASKED_EQ, 0x00ff, 1),
                       Arg(2, EQ, 2))
    f.add_rule_exactly(ALLOW, 1000,
                       Arg(0, EQ, 0),
                       Arg(1, MASKED_EQ, 0xffff, 11),
                       Arg(2, EQ, 2))
    f.add_rule_exactly(ALLOW, 1000,
                       Arg(0, EQ, 0),
                       Arg(1, MASKED_EQ, 0xffff, 111),
                       Arg(2, EQ, 2))
    f.add_rule_exactly(ALLOW, 1000,
                       Arg(0, EQ, 0),
                       Arg(1, MASKED_EQ, 0xff00, 1000),
                       Arg(2, EQ, 2))
    return f

args = util.get_opt()
ctx = test(args)
util.filter_output(args, ctx)

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
