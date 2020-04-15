#!/bin/sh
# the first parameter is the interface,
# the second parameter is the % chance any given packet is dropped
tc qdisc add dev $1 root netem loss $2%
