The usage of this command is: `ping [-6] [-i interval] [-W timeout] [-s
bodysize] {destination}`

If running a Linux system, you can run the `packet-loss-simulator.sh` script
with the first argument being the interface and the second being the percentage
chance of any given packet being dropped, e.g. `packet-loss-simulator.sh eth0
15`. Then you can restore your interface with the `packet-loss-simulator.sh`
which takes the interface as its first and only argument.
