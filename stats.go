package main

import (
	"fmt"
	"time"
)

func round(val float64) int {
	// Go seemed a sweet language in the beginning...
	if val < 0 {
		return int(val - 0.5)
	}
	return int(val + 0.5)
}

func displayStats(channel <-chan result) {
	// Displays every N seconds the number of sent requests, and the rate
	start := time.Now()
	sent := 0
	errors := 0
	total := 0
	for {
		// Read the channel and add the number of sent messages
		added := <-channel
		sent += added.sent
		errors += added.err

		if added.sent == 0 {
			// Something has asked for a display flush

			elapsedSeconds := float64(time.Since(start)) / float64(time.Second)

			fmt.Printf(
				"Requests sent: %dr/s\t(%d total)",
				round(float64(sent)/elapsedSeconds),
				total+sent,
			)
			// Successful requests? (replies received)
			fmt.Printf(
				"\tReplies received: %dr/s",
				round(float64(sent-errors)/elapsedSeconds),
			)

			if errors > 0 {
				fmt.Printf(
					"\t Errors: %d (%d%%)",
					errors,
					100*errors/sent,
				)
			}
			fmt.Print("\n")

			start = time.Now()
			total += sent
			sent = 0
			errors = 0
		}
	}
}

func timerStats(channel chan<- result) {
	// Periodically triggers a display update for the stats
	for {
		timer := time.NewTimer(time.Duration(displayInterval) * time.Millisecond)
		<-timer.C
		channel <- result{0, 0}
	}
}
