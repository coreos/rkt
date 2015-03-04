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
    # the next three seccomp_rule_add_exact() calls for read must go together
    # in this order to catch an infinite loop.
    f.add_rule(ALLOW, "read", Arg(0, EQ, sys.stdout.fileno()))
    f.add_rule(ALLOW, "read", Arg(1, EQ, 0))
    f.add_rule(ALLOW, "read", Arg(0, EQ, sys.stdin.fileno()))
    return f

args = util.get_opt()
ctx = test(args)
util.filter_output(args, ctx)

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
