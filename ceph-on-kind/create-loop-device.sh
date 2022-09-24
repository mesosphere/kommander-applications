#!/usr/bin/env bash

cd /tmp
mkdir -p dkp
dd if=/dev/zero of=dkp/vol1 bs=1G count=10
losetup -f --show /tmp/dkp/vol1
