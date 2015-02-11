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
    f.remove_arch(Arch())
    f.add_arch(Arch("x86"))
    f.add_arch(Arch("x86_64"))
    f.add_arch(Arch("x32"))
    f.add_arch(Arch("arm"))
    f.add_arch(Arch("aarch64"))
    f.add_arch(Arch("mipsel"))
    f.add_arch(Arch("mipsel64"))
    f.add_arch(Arch("mipsel64n32"))
    f.add_rule(ALLOW, "read", Arg(0, EQ, sys.stdin.fileno()))
    f.add_rule(ALLOW, "write", Arg(0, EQ, sys.stdout.fileno()))
    f.add_rule(ALLOW, "write", Arg(0, EQ, sys.stderr.fileno()))
    f.add_rule(ALLOW, "close")
    f.add_rule(ALLOW, "rt_sigreturn")
    return f

args = util.get_opt()
ctx = test(args)
util.filter_output(args, ctx)

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
