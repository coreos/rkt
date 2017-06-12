#!/bin/sh
set -e
set -x

grep -q file1 file1
grep -q file1 file2
grep -q file3 file3
grep -q file4 file4
if [ "$CHECK" != "rkt-rendered" ] ; then
	# Skip this test because of:
	# https://github.com/coreos/rkt/issues/1774
	test $(ls -i file1 |awk '{print $1}') -eq $(ls -i file2 |awk '{print $1}')
	test $(ls -i file3 |awk '{print $1}') -ne $(ls -i file4 |awk '{print $1}')
fi
echo "SUCCESS"
