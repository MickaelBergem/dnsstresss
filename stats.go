package main

import (
	"fmt"
	"time"

	"github.com/logrusorgru/aurora"
)

func round(val float64) int {
	// Go seemed a sweet language in the beginning...
	if val < 0 {
		return int(val - 0.5)
	}
	return int(val + 0.5)
}

type statsMessage struct {
	sent       int
	err        int
	flush      bool
	elapsed    time.Duration
	maxElapsed time.Duration
}

func displayStats(channel chan statsMessage) {
	// Displays every N seconds the number of sent requests, and the rate
	start := time.Now()
	sent := 0
	var elapsed time.Duration
	var maxElapsed time.Duration
	errors := 0
	total := 0
	for {
		// Read the channel and add the number of sent messages
		added := <-channel
		sent += added.sent
		errors += added.err
		elapsed += added.elapsed
		if added.maxElapsed > maxElapsed {
			maxElapsed = added.maxElapsed
		}

		if added.flush == true {
			// Something has asked for a display flush

			elapsedSeconds := time.Since(start).Seconds()

			if sent > 0 {
				fmt.Printf(
					"Requests sent: %6.dr/s",
					round(float64(sent)/elapsedSeconds),
				)

				// Successful requests? (replies received)
				fmt.Printf(
					"\tReplies received: %6.dr/s",
					round(float64(sent-errors)/elapsedSeconds),
				)

				fmt.Printf(
					" (mean=%.0fms / max=%.0fms)",
					1000.*elapsed.Seconds()/float64(sent),
					1000.*maxElapsed.Seconds(),
				)

				if errors > 0 {
					fmt.Printf(
						"\t %s",
						aurora.Red(fmt.Sprintf("Errors: %d (%d%%)",
							errors,
							100*errors/sent,
						)),
					)
				}
			} else {
				fmt.Printf("No requests were sent.")
			}

			fmt.Print("\n")

			start = time.Now()
			total += sent
			sent = 0
			errors = 0
			elapsed = 0
			maxElapsed = 0
		}
	}
}

func timerStats(channel chan<- statsMessage) {
	// Periodically triggers a display update for the stats
	for {
		timer := time.NewTimer(time.Duration(displayInterval) * time.Millisecond)
		<-timer.C
		channel <- statsMessage{flush: true}
	}
}
