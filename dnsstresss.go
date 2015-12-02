package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"math/big"
	"net"
	"os"
	"strings"
)

// Runtime options
var (
	concurrency     int
	displayInterval int
	verbose         bool
	iterative       bool
	resolver        string
	randomIds       bool
	flood           bool
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
	flag.BoolVar(&flood, "f", false,
		"Don't wait for an answer before sending another")
}

type result struct {
	sent int
	err  int
}

type ControlMsg uint

const (
	DoFlush ControlMsg = iota
	DoTotal
	DoClose
)

func main() {
	fmt.Printf("dnsstresss - dns stress tool\n")

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
	// all remaining parameters are treated as domains to be used in round-robin in the threads
	targetDomains := make([]string, flag.NArg())
	for index, element := range flag.Args() {
		if element[len(element)-1] == '.' {
			targetDomains[index] = element
		} else {
			targetDomains[index] = element + "."
		}
	}

	fmt.Printf("Queried domains: %v.\n", targetDomains)

	// Create a channel for communicating the number of sent messages
	sentCounterCh := make(chan result, concurrency)

	controlChannel := make(chan ControlMsg)
	if !flood {
		go timerStats(controlChannel)
		fmt.Printf("Started timer channel.\n")
	} else {
		fmt.Println("Flooding mode, nothing will be printed.")
	}
	// We still need this useless routine to empty the channels, even when flooding
	go displayStats(sentCounterCh, controlChannel)

	// Run concurrently
	for threadID := 0; threadID < concurrency; threadID++ {
		go linearResolver(threadID, targetDomains[threadID%len(targetDomains)], sentCounterCh)
	}
	fmt.Printf("Started %d threads.\n", concurrency)

	fmt.Print("Press ENTER to quit")
	fmt.Scanln()

	controlChannel <- DoTotal
	controlChannel <- DoClose
}

func linearResolver(threadID int, domain string, sentCounterCh chan<- result) {
	// Resolve the domain as fast as possible
	if verbose {
		fmt.Printf("Starting thread #%d.\n", threadID)
	}

	// Every N steps, we will tell the stats module how many requests we sent
	displayStep := 5
	maxRequestID := big.NewInt(65536)
	errors := 0

	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	if iterative {
		message.RecursionDesired = false
	}

	for {
		nbSent := 0
		var lastErr error
		for i := 0; i < displayStep; i++ {
			// Try to resolve the domain
			if randomIds {
				// Regenerate message Id to avoid servers dropping (seemingly) duplicate messages
				newid, _ := rand.Int(rand.Reader, maxRequestID)
				message.Id = uint16(newid.Int64())
			}

			if flood {
				go dnsExchange(resolver, message)
			} else {
				sent, err := dnsExchange(resolver, message)
				if err != nil {
					if verbose {
						fmt.Printf("%s error: % (%s)\n", domain, err, resolver)
					}
					errors++
					lastErr = err
				}
				nbSent += sent
			}
		}

		// If sending doesn't work, there is probably a non-network error, so we abort and print the error
		if nbSent == 0 && errors > 0 {
			fmt.Printf("%s error: % (%s)\n", domain, lastErr, resolver)
			return
		}

		// Update the counter of sent requests and requests
		sentCounterCh <- result{nbSent, errors}
		errors = 0
	}
}

func dnsExchange(resolver string, message *dns.Msg) (int, error) {
	//XXX: How can we share the connection between subsequent attempts ?
	dnsconn, err := net.Dial("udp", resolver)
	if err != nil {
		return 0, err
	}
	co := &dns.Conn{Conn: dnsconn}
	defer co.Close()

	// Actually send the message and wait for answer
	co.WriteMsg(message)
	if err != nil {
		return 0, err
	}

	_, err = co.ReadMsg()
	return 1, err
}
