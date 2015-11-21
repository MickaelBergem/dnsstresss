package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"math/big"
	"os"
	"strings"
	"time"
)

// Runtime options
var (
	concurrency     int
	displayInterval int
	verbose         bool
	iterative       bool
	resolver        string
	randomIds       bool
)

func init() {
	flag.IntVar(&concurrency, "concurrency", 50,
		"Internal buffer")
	flag.IntVar(&displayInterval, "d", 1000,
		"Update interval of the stats (in ms)")
	flag.BoolVar(&verbose, "v", false,
		"Verbose logging")
	flag.BoolVar(&randomIds, "random", false,
		"Use random Request Identifiers for each query")
	flag.BoolVar(&iterative, "i", false,
		"Do an iterative query instead of recursive (to stress authoritative nameservers)")
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
			"Usage: resolve [option ...] targetdomain [targetdomain [...] ]",
			"",
		}, "\n"))
		flag.PrintDefaults()
	}

	flag.Parse()

	// all remaining parameters are treated as domains to be used in round-robin in the threads
	targetDomains := flag.Args()

	fmt.Printf("Queried domains are %v.\n", targetDomains)

	// Create a channel for communicating the number of sent messages
	sentCounterCh := make(chan result, concurrency)

	// Run concurrently
	for threadID := 0; threadID < concurrency; threadID++ {
		go linearResolver(threadID, targetDomains[threadID%len(targetDomains)], sentCounterCh)
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

func linearResolver(threadID int, domain string, sentCounterCh chan<- result) {
	// Resolve the domain as fast as possible
	if verbose {
		fmt.Printf("Starting thread #%d.\n", threadID)
	}

	// Every N steps, we will tell the stats module how many requests we sent
	displayStep := 5
	maxRequestId := big.NewInt(65536)
	errors := 0

	client := new(dns.Client)
	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	if iterative {
		message.RecursionDesired = false
	}

	for {
		for i := 0; i < displayStep; i++ {
			// Try to resolve the domain
			if randomIds {
				// Regenerate message Id to avoid servers dropping (seemingly) duplicate messages
				newid, _ := rand.Int(rand.Reader, maxRequestId)
				message.Id = uint16(newid.Int64())
			}
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
