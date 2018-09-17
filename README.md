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
    Send DNS requests as fast as possible to a given server and display the rate.

    Usage: dnsstresss [option ...] targetdomain
    -concurrency=50: Internal buffer
    -d=1000: Update interval of the stats (in ms)
    -r="127.0.0.1:53": Resolver to test against
    -v=false: Verbose logging

For IPv6 resolvers, use brackets and quotes:

    ./dnsstresss -r "[2001:4860:4860::8888]:53" -v google.com.

Example:

![GIF animation](example.gif)
