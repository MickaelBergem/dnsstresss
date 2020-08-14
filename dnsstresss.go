package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/logrusorgru/aurora"
	"github.com/miekg/dns"
)

var (
	concurrency   int
	flushInterval int
	verbose       bool
	iterative     bool
	resolver      string
	randomIds     bool
	flood         bool
	au            aurora.Aurora
	DatadogStatsd *statsd.Client

)

func init() {
	flag.IntVar(&concurrency, "concurrency", 50,
		"Internal buffer")
	flag.IntVar(&flushInterval, "l", 1000,
		"flush interval of the stats (in ms)")
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
	hasErrors := false
	for i := range targetDomains {
		hasErrors = hasErrors || testRequest(targetDomains[i])
	}
	if hasErrors {
		fmt.Printf("%s %s", au.BgYellow(" WARNING "), "Could not resolve some domains you provided, you may receive only errors.\n")
	}

	// Create a channel for communicating the number of sent messages
	sentCounterCh := make(chan statsMessage, concurrency)

	// Run concurrently
	for threadID := 0; threadID < concurrency; threadID++ {
		go linearResolver(threadID, targetDomains[threadID%len(targetDomains)], sentCounterCh)
	}
	fmt.Print(au.Blue(fmt.Sprintf("Started %d threads.\n", concurrency)))

	if !flood {
		go timerStats(sentCounterCh)
	} else {
		fmt.Println("Flooding mode, nothing will be printed.")
	}
	// We still need this useless routine to empty the channels, even when flooding
	flushStats(sentCounterCh)
}

func testRequest(domain string) bool {
	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	if iterative {
		message.RecursionDesired = false
	}
	_, err := dnsExchange(resolver, message)
	if err != nil {
		fmt.Printf("Checking \"%s\" failed: %+v (using %s)\n", domain, au.Red(err), resolver)
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
	// Calculated from the length of the dns message sent
	bytesSent := 0

	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	if iterative {
		message.RecursionDesired = false
	}

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
				len, err := dnsExchange(resolver, message)
				bytesSent += len
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
			sent:          displayStep,
			err:           errors,
			bytesSent: bytesSent,
		}
		errors = 0
		bytesSent = 0
	}
}

func dnsExchange(resolver string, message *dns.Msg) (int, error) {
	dnsconn, err := net.Dial("udp", resolver)
	if err != nil {
		return 0, err
	}
	co := &dns.Conn{Conn: dnsconn}
	defer co.Close()

	// Actually send the message and wait for answer
	co.WriteMsg(message)

	_, err = co.ReadMsg()

	return message.Len(), err
}
