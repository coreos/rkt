# rkt-monitor

This is a small go utility intended to monitor the CPU and memory usage of rkt
and its children processes. This is accomplished by exec'ing rkt, reading proc
once a second for a specified duration, and printing the results.

This utility has a handful of flags:

```
Usage:
  rkt-monitor IMAGE [flags]

Examples:
rkt-monitor mem-stresser.aci -v -d 30s

Flags:
  -f, --to-file[=false]: Save benchmark results to files in a temp dir
  -w, --output-dir="/tmp": Specify directory to write results
  -p, --rkt-dir="": Directory with rkt binary
  -s, --stage1-path="": Path to Stage1 image to use, default: coreos
  -d, --duration="10s": How long to run the ACI
  -h, --help[=false]: help for rkt-monitor
  -r, --repetitions=1: Numbers of benchmark repetitions
  -o, --show-output[=false]: Display rkt's stdout and stderr
  -v, --verbose[=false]: Print current usage every second
```

Some acbuild scripts and golang source code is provided to build ACIs that
attempt to eat up resources in different ways.

An example usage:

```
$ ./tests/rkt-monitor/build-stresser.sh log
Building worker...
Beginning build with an empty ACI
Setting name of ACI to appc.io/rkt-log-stresser
Copying host:worker-binary to aci:/worker
Setting exec command [/worker]
Writing ACI to log-stresser.aci
Ending the build
$ sudo ./build-rkt-1.10.0+git/target/bin/rkt-monitor log-stresser.aci 
[sudo] password for derek: 
rkt(13261): seconds alive: 10  avg CPU: 33.113897%  avg Mem: 4 kB  peak Mem: 4 kB
systemd(13302): seconds alive: 9  avg CPU: 0.000000%  avg Mem: 4 mB  peak Mem: 4 mB
systemd-journal(13303): seconds alive: 9  avg CPU: 68.004584%  avg Mem: 7 mB  peak Mem: 7 mB
worker(13307): seconds alive: 9  avg CPU: 13.004088%  avg Mem: 1 mB  peak Mem: 1 mB
load average in a container: Load1: 0.280000 Load5: 0.250000 Load15: 0.200000
container start time: 315621ns
container stop time: 17280555ns
```
