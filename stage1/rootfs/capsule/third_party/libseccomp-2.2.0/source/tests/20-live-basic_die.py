#!/usr/bin/env python

#
# Seccomp Library test program
#
# Copyright (c) 2013 Red Hat <pmoore@redhat.com>
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
    action = util.parse_action(sys.argv[1])
    if action == TRAP:
        util.install_trap()
    f = SyscallFilter(action)
    f.add_rule(ALLOW, "rt_sigreturn")
    f.add_rule(ALLOW, "exit_group")
    f.load()
    try:
        util.write_file("/dev/null")
    except OSError as ex:
        quit(ex.errno)
    quit(160)

test()

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
