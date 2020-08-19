package main

import (
	"flag"
	"fmt"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/logrusorgru/aurora"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

var (
	concurrency   int
	flushInterval int
	maxMessages   int
	verbose       bool
	iterative     bool
	resolver      string
	randomIds     bool
	flood         bool
	runForever    bool
	au            aurora.Aurora
	DatadogStatsd *statsd.Client

)

func init() {
	flag.IntVar(&concurrency, "concurrency", 50,
		"Internal buffer")
	flag.IntVar(&maxMessages, "m", 10000,
		"Maximum number of messages to send before stopping. Can be overriden to never stop with -inf")
	flag.BoolVar(&verbose, "v", false,
		"Verbose logging")
	flag.BoolVar(&randomIds, "random", false,
		"Use random Request Identifiers for each query")
	flag.BoolVar(&iterative, "i", false,
		"Do an iterative query instead of recursive (to stress authoritative nameservers)")
	flag.StringVar(&resolver, "r", "127.0.0.1:53",
		"Resolver to test against")
	flag.BoolVar(&flood, "f", false,
		"Don't wait for an answer before sending another")
	flag.BoolVar(&runForever, "inf", false,
		"Run Forever")
	DatadogStatsd = InitApp()
}

func InitApp() *statsd.Client{
	statsd, err := statsd.New("127.0.0.1:8125")
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return statsd
}

func main() {
	fmt.Printf("dnsstresss - dns stress tool\n\n")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, strings.Join([]string{
			"Send DNS requests as fast as possible to a given server and display the rate.",
			"",
			"Usage: dnsstresss [option ...] targetdomain [targetdomain [...] ]",
			"",
		}, "\n"))
		flag.PrintDefaults()
	}

	flag.Parse()

	// We need at least one target domain
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	au = aurora.NewAurora(true)

	if !strings.Contains(resolver, ":") { // TODO: improve this test to make it work with IPv6 addresses
		// Automatically append the default port number if missing
		resolver = resolver + ":53"
	}

	// all remaining parameters are treated as domains to be used in round-robin in the threads
	targetDomains := make([]string, flag.NArg())
	for index, element := range flag.Args() {
		if element[len(element)-1] == '.' {
			targetDomains[index] = element
		} else {
			targetDomains[index] = element + "."
		}
	}

	fmt.Printf("Target domains: %v.\n", targetDomains)

	exit :=  make(chan struct{})
	dnsResolver := NewResolver(resolver, targetDomains[0], concurrency, flood, DatadogStatsd, exit)
	dnsResolver.RunResolver()
	defer dnsResolver.Close()

	for {
		select {
		case <-exit:
			close(exit)
			return
		default:
			if int64(maxMessages) < atomic.LoadInt64(&dnsResolver.totalSent) && !runForever {
				fmt.Println("Sent %d messages, and %d bytes", atomic.LoadInt64(&dnsResolver.totalSent), dnsResolver.totalBytesSent)
				close(exit)
				return
			}
			continue
		}
	}
}

