package main

import (
	"fmt"
	"time"
)

type statsMessage struct {
	sent       int
	err        int
	bytesSent  int
	flush      bool
}

func flushStats(channel chan statsMessage) {
	sent := 0
	errors := 0
	total := 0
	bytesSent := 0
	totalBytesSent := 0

	for {
		// Read the channel and add the number of sent messages
		added := <-channel
		sent += added.sent
		errors += added.err
		bytesSent += added.bytesSent

		if added.flush == true {
			// Something has asked for a display flush

			if sent > 0 {
				DatadogStatsd.Count("npm.udp.testing.sent_packets", int64(sent), nil, 1)

				DatadogStatsd.Count("npm.udp.testing.successful_requests", int64(sent-errors), nil, 1)

				DatadogStatsd.Count("npm.udp.testing.bytes_sent", int64(bytesSent), nil, 1)

				if errors > 0 {
					DatadogStatsd.Count("npm.udp.testing.errors", int64(errors), nil, 1)
				}
			} else {
				fmt.Printf("No requests were sent.")
			}

			total += sent
			totalBytesSent += bytesSent
			sent = 0
			errors = 0
			bytesSent = 0
		}
	}
}

func timerStats(channel chan<- statsMessage) {
	// Triggers a stat flush every second
	for {
		timer := time.NewTimer(time.Duration(flushInterval) * time.Millisecond)
		<-timer.C
		channel <- statsMessage{flush: true}
	}
}
