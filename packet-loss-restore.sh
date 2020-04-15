#!/bin/sh
# the first parameter is the interface,
# the second parameter is the % chance any given packet is dropped
tc qdisc del dev $1 root
