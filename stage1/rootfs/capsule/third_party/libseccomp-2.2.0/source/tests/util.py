#
# Seccomp Library utility code for tests
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

""" Python utility code for the libseccomp test suite """

import argparse
import os
import sys
import signal

from seccomp import *

def trap_handler(signum, frame):
    """ SIGSYS signal handler, internal use only
    """
    os._exit(161)

def get_opt():
    """ Parse the arguments passed to main

    Description:
    Parse the arguments passed to the test from the command line.  Returns
    a parsed argparse object.
    """
    parser = argparse.ArgumentParser()
    parser.add_argument("-b", "--bpf", action="store_true")
    parser.add_argument("-p", "--pfc", action="store_true")
    return parser.parse_args()

def filter_output(args, ctx):
    """ Output the filter in either BPF or PFC

    Arguments:
    args - an argparse object from UtilGetOpt()
    ctx - a seccomp SyscallFilter object

    Description:
    Output the SyscallFilter to stdout in either BPF or PFC format depending
    on the test's command line arguments.
    """
    if (args.bpf):
        ctx.export_bpf(sys.stdout)
    else:
        ctx.export_pfc(sys.stdout)

def install_trap():
    """ Install a TRAP action signal handler

    Description:
    Install the TRAP action signal handler.
    """
    signal.signal(signal.SIGSYS, trap_handler)

def parse_action(action):
    """ Parse a filter action string into an action value

    Arguments:
    action - the action string

    Description:
    Parse a seccomp action string into the associated integer value.
    """
    if action == "KILL":
        return KILL
    elif action == "TRAP":
        return TRAP
    elif action == "ERRNO":
        return ERRNO(163)
    elif action == "TRACE":
        raise RuntimeError("the TRACE action is not currently supported")
    elif action == "ALLOW":
        return ALLOW
    raise RuntimeError("invalid action string")


def write_file(path):
    """ Write a string to a file

    Arguments:
    path - the file path

    Description:
    Open the specified file, write a string to the file, and close the file.
    """
    fd = os.open(path, os.O_WRONLY|os.O_CREAT, 0600)
    if not os.write(fd, "testing") == len("testing"):
        raise IOError("failed to write the full test string in write_file()")
    os.close(fd)

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
