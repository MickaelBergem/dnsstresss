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

func displayStats(channel <-chan result, controlChannel <-chan ControlMsg) {
	// Displays every N seconds the number of sent requests, and the rate
	start := time.Now()
	last := start
	sent := 0
	errors := 0
	total := 0
	for {
		select {
		case msg := <-controlChannel:
			if msg == DoClose {
				return
			} else {
				var elapsedSeconds float64
				total += sent
				if msg == DoTotal {
					elapsedSeconds = float64(time.Since(start)) / float64(time.Second)
					sent = total
					fmt.Printf(
						"Total requests sent: %d\t(%dr/s)",
						total,
						round(float64(total)/elapsedSeconds),
					)
				} else if msg == DoFlush {
					elapsedSeconds = float64(time.Since(last)) / float64(time.Second)
					fmt.Printf(
						"Requests sent: %dr/s\t(%d total)",
						round(float64(sent)/elapsedSeconds),
						total,
					)
					// Successful requests? (replies received)
					if sent > 0 {
						fmt.Printf(
							"\tReplies received: %dr/s",
							round(float64(sent-errors)/elapsedSeconds),
						)
					}
					if errors > 0 {
						pct := 0
						if sent > 0 {
							pct = 100 * errors / sent
						} else {
							pct = 100
						}
						fmt.Printf(
							"\t Errors: %d (%d%%)",
							errors,
							pct,
						)
					}
				}
				fmt.Print("\n")
				last = time.Now()
				sent = 0
				errors = 0
			}
		// Read the channel and add the number of sent messages
		case added := <-channel:
			sent += added.sent
			errors += added.err
		}
	}
}

func timerStats(channel chan<- ControlMsg) {
	// Periodically triggers a display update for the stats
	for {
		timer := time.NewTimer(time.Duration(displayInterval) * time.Millisecond)
		<-timer.C
		channel <- DoFlush
	}
}
