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
	verbosity       int
	iterative       bool
	resolver        string
	randomIds       bool
	flood           bool
	targetDomains   []string
)

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

const (
	Verb_ERR = iota
	Verb_WARN
	Verb_NOTICE
	Verb_INFO
	Verb_DEBUG
)

func main() {

	parseCommandLine()

	// Create a channel for communicating the number of sent messages
	controlChannel := make(chan ControlMsg)
	defer close(controlChannel)

	if flood {
		go runFlood()
	} else {
		sentCounterCh := make(chan result, concurrency)
		defer close(sentCounterCh)
		go timerStats(controlChannel)
		go displayStats(sentCounterCh, controlChannel)
		go runWorkers(sentCounterCh)
		if verbosity >= Verb_DEBUG {
			fmt.Println("Started timer channel.")
		}
	}

	fmt.Println("Press ENTER to quit")
	fmt.Scanln()

	if !flood {
		controlChannel <- DoTotal
		controlChannel <- DoClose
	}
}

func runWorkers(sharedChannel chan result) {

	for threadID := 0; threadID < concurrency; threadID++ {
		go linearResolver(threadID, targetDomains[threadID%len(targetDomains)], sharedChannel)
	}
	if verbosity >= Verb_DEBUG {
		fmt.Printf("Started %d threads.\n", concurrency)
	}
}

func initConn() (*dns.Conn, error) {
	dnsconn, err := net.Dial("udp", resolver)
	if err != nil {
		if verbosity >= Verb_WARN {
			fmt.Printf("%s error: % (%s)\n", err, resolver)
		}
		return nil, err
	}
	co := &dns.Conn{Conn: dnsconn}
	return co, nil
}

func composeMessage(domain string) *dns.Msg {
	message := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	if iterative {
		message.RecursionDesired = false
	}
	return message
}

func linearResolver(threadID int, domain string, sentCounterCh chan<- result) {
	if verbosity >= Verb_DEBUG {
		fmt.Printf("Starting thread #%d.\n", threadID)
	}

	// Every N steps, we will tell the stats module how many requests we sent
	displayStep := 5
	maxRequestID := big.NewInt(65536)
	errors := 0

	if co, err := initConn(); err != nil {
		sentCounterCh <- result{0, 1}
		return
	} else {
		defer co.Close()

		message := composeMessage(domain)

		for {
			nbSent := 0
			for i := 0; i < displayStep; i++ {
				// Try to resolve the domain
				if randomIds {
					// Regenerate message Id to avoid servers dropping (seemingly) duplicate messages
					newid, _ := rand.Int(rand.Reader, maxRequestID)
					message.Id = uint16(newid.Int64())
				}

				// Actually send the message and wait for answer
				err = co.WriteMsg(message)
				if err != nil {
					if verbosity >= Verb_ERR {
						fmt.Printf("%s error: % (%s)\n", domain, err, resolver)
					}
					sentCounterCh <- result{0, 1}
					return
				}
				nbSent++

				_, err = co.ReadMsg()
				if err != nil {
					if verbosity >= Verb_DEBUG {
						fmt.Printf("%s error: % (%s)\n", domain, err, resolver)
					}
					errors++
				}
				err = nil
			}

			// Update the counter of sent requests and requests
			sentCounterCh <- result{nbSent, errors}
			errors = 0
		}
	}
}

func parseCommandLine() {
	flag.IntVar(&concurrency, "concurrency", 50,
		"Internal buffer")
	flag.IntVar(&displayInterval, "d", 1000,
		"Update interval of the stats (in ms)")
	verbose := flag.Bool("v", false, "Verbose logging")
	quiet := flag.Bool("q", false, "Less logging")
	flag.BoolVar(&randomIds, "random", false,
		"Use random Request Identifiers for each query")
	flag.BoolVar(&iterative, "i", false,
		"Do an iterative query instead of recursive (to stress authoritative nameservers)")
	flag.StringVar(&resolver, "r", "127.0.0.1:53",
		"Resolver to test against")
	flag.BoolVar(&flood, "f", false,
		"Don't wait for an answer before sending another")

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

	if *verbose {
		verbosity = Verb_DEBUG
	} else if *quiet {
		verbosity = Verb_WARN
	} else {
		verbosity = Verb_INFO
	}

	if verbosity >= Verb_INFO {
		fmt.Println("dnsstresss - dns stress tool")
	}

	// We need at least one target domain
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	// all remaining parameters are treated as domains to be used in round-robin in the threads
	targetDomains = make([]string, flag.NArg())
	for index, element := range flag.Args() {
		if element[len(element)-1] == '.' {
			targetDomains[index] = element
		} else {
			targetDomains[index] = element + "."
		}
	}

	if verbosity >= Verb_INFO {
		fmt.Printf("Queried domains: %v.\n", targetDomains)
	}

}

func runFlood() {
	for threadID := 0; threadID < concurrency; threadID++ {
		go floodNoWait(targetDomains[threadID%len(targetDomains)])
	}
}

func floodNoWait(domain string) {
	if co, err := initConn(); err != nil {
		return
	} else {
		defer co.Close()
		message := composeMessage(domain)
		for {
			co.WriteMsg(message)
		}
	}
}
