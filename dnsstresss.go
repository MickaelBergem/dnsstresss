package main

import (
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"os"
	"strings"
	"time"
)

// Runtime options
var concurrency int
var displayInterval int
var verbose bool
var targetDomain string
var resolver string

func init() {
	flag.IntVar(&concurrency, "concurrency", 50,
		"Internal buffer")
	flag.IntVar(&displayInterval, "d", 1000,
		"Update interval of the stats (in ms)")
	flag.BoolVar(&verbose, "v", false,
		"Verbose logging")
	flag.StringVar(&resolver, "r", "127.0.0.1:53",
		"Resolver to test against")
}

type result struct {
	sent int
	err  int
}

func main() {
	fmt.Printf("dnsstresss - dns stress tool\n")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, strings.Join([]string{
			"\"resolve\" mass resolve DNS A records for domains names read from stdin.",
			"",
			"Usage: resolve [option ...] targetdomain",
			"",
		}, "\n"))
		flag.PrintDefaults()
	}

	flag.Parse()

	// We need exactly one parameter (the target domain)
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	// The target domain should be the first (and only) parameter
	targetDomain := flag.Args()[0]

	fmt.Printf("Queried domain is %s.\n", targetDomain)

	// Create a channel for communicating the number of sent messages
	sentCounterCh := make(chan result, concurrency)

	// Run concurrently
	for threadID := 0; threadID < concurrency; threadID++ {
		go linearResolver(threadID, targetDomain, sentCounterCh)
		if concurrency <= 10000 {
			// Small delay so that the real-time stats are more accurate
			time.Sleep(1 * time.Millisecond)
		}
	}
	fmt.Printf("Started %d threads.\n", concurrency)

	go timerStats(sentCounterCh)
	fmt.Printf("Started timer channel.\n")

	displayStats(sentCounterCh)
}

func linearResolver(threadID int, domain string, sentCounterCh chan result) {
	// Resolve the domain as fast as possible
	if verbose {
		fmt.Printf("Starting thread #%d.\n", threadID)
	}

	// Every N steps, we will tell the stats module how many requests we sent
	displayStep := 5
	errors := 0

	client := new(dns.Client)
	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)

	for {
		for i := 0; i < displayStep; i++ {
			// Try to resolve the domain
			_, _, err := client.Exchange(message, resolver)
			if err != nil {
				if verbose {
					fmt.Printf("%s error: % (%s)\n", domain, err, resolver)
				}
				errors++
			}
		}

		// Update the counter of sent requests and requests
		sentCounterCh <- result{displayStep, errors}
		errors = 0
	}
}
