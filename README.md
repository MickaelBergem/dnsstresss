# DNSStresss, the DNS stress test tool

Simple Go program to stress test a DNS server.

It displays the number of queries made, along with the answer per second rate reached.

## Usage

First:

    go install github.com/MickaelBergem/dnsstresss@latest

Then:

    $ dnsstresss -h
    dnsstresss - dns stress tool

    Send DNS requests as fast as possible to a given server and display the rate.

    Usage: dnsstresss [option ...] targetdomain [targetdomain [...] ]
    -concurrency int
                Internal buffer (default 50)
    -d int      Update interval of the stats (in ms) (default 1000)
    -f          Don't wait for an answer before sending another
    -i          Do an iterative query instead of recursive (to stress authoritative nameservers)
    -r string   Resolver to test against (default "127.0.0.1:53")
    -random     Use random Request Identifiers for each query
    -v          Verbose logging

For IPv6 resolvers, use brackets and quotes:

    dnsstresss -r "[2001:4860:4860::8888]:53" -v google.com.

Example:

<p align="center">
    <img src="https://mickaelbergem.github.io/dnsstresss/animation.svg" alt="Usage of DNSStresss, the DNS stress test tool">
</p>
