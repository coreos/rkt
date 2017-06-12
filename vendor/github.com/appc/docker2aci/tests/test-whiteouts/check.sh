#!/bin/sh
set -e
set -x

grep -q yes layer0-file1
grep -q yes layer0-file2
grep -q yes layer0-file3

test ! -e layer1-file1
test   -e layer1-file2
test   -e layer1-file3

test   -e layer2-file1
test ! -e layer2-file2
test   -e layer2-file3

test   -e layer3-file1
test   -e layer3-file2
test ! -e layer3-file3

grep -q yes layer1-file2
grep -q yes layer1-file3

grep -q yes layer2-file1
grep -q yes layer2-file3

grep -q yes layer3-file1
grep -q yes layer3-file2


test   -e layer4-file1
test ! -e layer4-file2
test ! -e layer4-file3

test ! -e layer5-file1
test   -e layer5-file2
test ! -e layer5-file3

test ! -e layer6-file1
test ! -e layer6-file2
test   -e layer6-file3

grep -q yes layer4-file1
grep -q yes layer5-file2
grep -q yes layer6-file3


grep -q NEW layer10-file1
grep -q NEW layer10-file2
grep -q NEW layer10-file3

grep -q line1 layer11-file1
grep -q line1 layer11-file2
grep -q line1 layer11-file3

# # Docker with AUFS or overlay storage backend does not handle this test
# # correctly and Semaphore uses AUFS
if [ "$DOCKER_STORAGE_BACKEND" == devicemapper ] ; then
	grep -q line2 layer11-file1
	grep -q line2 layer11-file2
	grep -q line2 layer11-file3
	cmp layer11-file1 layer11-file2
	cmp layer11-file1 layer11-file3
fi

echo "SUCCESS"
