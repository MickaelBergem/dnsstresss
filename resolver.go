package main

import (
	"crypto/rand"
	"fmt"
	"github.com/DataDog/datadog-go/statsd"
	"github.com/miekg/dns"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var MaxRequestID = big.NewInt(65536)

//TODO: Add function to test if resolver is working
type Resolver struct {
	sent      int64
	errors    int64
	bytesSent int64

	totalSent      int64
	totalErrors    int64
	totalBytesSent int64

	concurrency    int
	server         string
	stopChan       chan struct{}
	message        *dns.Msg
	msgBuf         []byte
	statsdReporter *statsd.Client

	flood bool
}

func NewResolver(server string, domain string, concurrency int, flood bool, client *statsd.Client, exit chan struct{}) *Resolver {
	msg := new(dns.Msg).SetQuestion(domain, dns.TypeA)
	msgBuf, err := msg.Pack()
	if err != nil {
		return nil
	}

	r := &Resolver{
		server:         server,
		sent:           0,
		errors:         0,
		bytesSent:      0,
		totalSent:      0,
		totalErrors:    0,
		totalBytesSent: 0,
		flood:          flood,
		concurrency:    concurrency,
		statsdReporter: client,
		stopChan:       exit,
		message:        msg,
		msgBuf:         msgBuf,
	}

	go r.flushStats(r.stopChan)
	return r
}

func (r *Resolver) Close() {
	close(r.stopChan)
}

func (r *Resolver) RunResolver() {
	for i := 0; i < r.concurrency; i++ {

		raddr, err := net.ResolveUDPAddr("udp", r.server)
		if err != nil {
			return
		}

		// Create connection that we will re-use to write messages
		// This way we do not create too many connections and run out
		// of ports
		conn, err := net.DialUDP("udp", nil, raddr)
		if err != nil {
			return
		}
		defer conn.Close()
		go r.resolve(r.stopChan, conn)
	}
}

func (r *Resolver) resolve(exit <-chan struct{}, udpConn *net.UDPConn) {
	for {
		select {
		case <-exit:
			return
		default:
			if r.flood {
				var wg sync.WaitGroup
				for i := 0; i < 100; i++ {
					wg.Add(1)
					go r.waitExchange(&wg, udpConn)
				}
				wg.Wait()
			} else {
				r.exchange(udpConn)
				r.updateMessageID()
			}
		}
	}
}

func (r *Resolver) waitExchange(wg *sync.WaitGroup, udpConn *net.UDPConn) error {
	r.exchange(udpConn)
	wg.Done()
	return nil
}

func (r *Resolver) exchange(udpConn *net.UDPConn) error {
	// Actually send the message and wait for answer
	udpConn.Write(r.msgBuf)
	atomic.AddInt64(&r.sent, 1)
	atomic.AddInt64(&r.bytesSent, int64(len(r.msgBuf)))
	return nil
}

func (r *Resolver) updateMessageID() {
	newid, _ := rand.Int(rand.Reader, MaxRequestID)
	r.message.Id = uint16(newid.Int64())
}

func (r *Resolver) flushStats(exit <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	// starts running the body immediately instead waiting for the first tick
	for range ticker.C {
		select {
		case <-exit:
			return
		default:

			// Load all the stats
			totalSent := atomic.LoadInt64(&r.totalSent)
			totalErrors := atomic.LoadInt64(&r.totalErrors)
			totalBytesSent := atomic.LoadInt64(&r.totalBytesSent)
			sent := atomic.LoadInt64(&r.sent)
			errors := atomic.LoadInt64(&r.errors)
			bytesSent := atomic.LoadInt64(&r.bytesSent)

			// Calculate increases since last report
			sentDelta := sent - totalSent
			errorDelta := errors - totalErrors
			bytesSentDelta := bytesSent - totalBytesSent

			// Submit stats
			err := r.statsdReporter.Count("npm.udp.testing.sent_packets", sentDelta, nil, 1)
			if err != nil {
				fmt.Print(err)
			}
			r.statsdReporter.Count("npm.udp.testing.successful_requests", sentDelta-errorDelta, nil, 1)
			r.statsdReporter.Count("npm.udp.testing.bytes_sent", bytesSentDelta, nil, 1)

			// Update totals
			atomic.AddInt64(&r.totalSent, sentDelta)
			atomic.AddInt64(&r.totalErrors, errorDelta)
			atomic.AddInt64(&r.totalBytesSent, bytesSentDelta)
		}
	}
}
