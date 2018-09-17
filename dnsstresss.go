package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/miekg/dns"
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

	if !strings.Contains(resolver, ":") {
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
	hasErrors := false
	for i := range targetDomains {
		hasErrors = hasErrors || testRequest(targetDomains[i])
	}
	if hasErrors {
		fmt.Printf("%s %s", aurora.BgBrown(" WARNING "), "Could not resolve some domains you provided, you may receive only errors.\n")
	}

	// Create a channel for communicating the number of sent messages
	sentCounterCh := make(chan statsMessage, concurrency)

	// Run concurrently
	for threadID := 0; threadID < concurrency; threadID++ {
		go linearResolver(threadID, targetDomains[threadID%len(targetDomains)], sentCounterCh)
	}
	fmt.Print(aurora.Gray(fmt.Sprintf("Started %d threads.\n", concurrency)))

	if !flood {
		go timerStats(sentCounterCh)
	} else {
		fmt.Println("Flooding mode, nothing will be printed.")
	}
	// We still need this useless routine to empty the channels, even when flooding
	displayStats(sentCounterCh)
}

func testRequest(domain string) bool {
	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	if iterative {
		message.RecursionDesired = false
	}
	err := dnsExchange(resolver, message)
	if err != nil {
		fmt.Printf("Checking \"%s\" failed: %+v (using %s)\n", domain, aurora.Red(err), resolver)
		return true
	}
	return false
}

func linearResolver(threadID int, domain string, sentCounterCh chan<- statsMessage) {
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

	var start time.Time
	var elapsed time.Duration    // Total time spent resolving
	var maxElapsed time.Duration // Maximum time took by a request

	for {
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
				start = time.Now()
				err := dnsExchange(resolver, message)
				spent := time.Since(start)
				elapsed += spent
				if spent > maxElapsed {
					maxElapsed = spent
				}
				if err != nil {
					if verbose {
						fmt.Printf("%s error: %d (%s)\n", domain, err, resolver)
					}
					errors++
				}
			}
		}

		// Update the counter of sent requests and requests
		sentCounterCh <- statsMessage{
			sent:       displayStep,
			err:        errors,
			elapsed:    elapsed,
			maxElapsed: maxElapsed,
		}
		errors = 0
		elapsed = 0
		maxElapsed = 0
	}
}

func dnsExchange(resolver string, message *dns.Msg) error {
	//XXX: How can we share the connection between subsequent attempts ?
	dnsconn, err := net.Dial("udp", resolver)
	if err != nil {
		return err
	}
	co := &dns.Conn{Conn: dnsconn}
	defer co.Close()

	// Actually send the message and wait for answer
	co.WriteMsg(message)

	_, err = co.ReadMsg()
	return err
}
