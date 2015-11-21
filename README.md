# DNSStresss

Simple Go program to stress test a DNS server.

It displays the number of queries made, along the rate reached.
For now, it blocks until it gets a response, so don't expect to efficiently DoS
a DNS server with this tool.

## Usage

First:

    go build

Then:

    ./dnsstresss -h
    dnsstresss - dns stress tool
    "resolve" mass resolve DNS A records for domains names read from stdin.

    Usage: resolve [option ...] targetdomain
      -concurrency=5000: Internal buffer
      -d=1000: Update interval of the stats (in ms)
      -v=false: Verbose logging

Example:

![GIF animation](example.gif)
