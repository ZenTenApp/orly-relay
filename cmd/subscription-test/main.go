package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"git.smesh.lol/orly/pkg/interfaces/neterr"
)

var (
	relayURL     = flag.String("url", "ws://localhost:3334", "Relay WebSocket URL")
	duration     = flag.Int("duration", 60, "Test duration in seconds")
	eventKind    = flag.Int("kind", 1, "Event kind to subscribe to")
	verbose      = flag.Bool("v", false, "Verbose output")
	subID        = flag.String("sub", "", "Subscription ID (default: auto-generated)")
)

type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

func main() {
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Generate subscription ID if not provided
	subscriptionID := *subID
	if subscriptionID == "" {
		subscriptionID = fmt.Sprintf("test-%d", time.Now().Unix())
	}

	log.Printf("Starting subscription stability test")
	log.Printf("Relay: %s", *relayURL)
	log.Printf("Duration: %d seconds", *duration)
	log.Printf("Event kind: %d", *eventKind)
	log.Printf("Subscription ID: %s", subscriptionID)
	log.Println()

	// Connect to relay
	log.Printf("Connecting to %s...", *relayURL)
	conn, _, err := websocket.DefaultDialer.Dial(*relayURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	log.Printf("✓ Connected")
	log.Println()

	// Context for the test
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*duration+10)*time.Second)
	defer cancel()

	// Handle interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\nInterrupted, shutting down...")
		cancel()
	}()

	// Counters
	var receivedCount atomic.Int64
	var lastEventTime atomic.Int64
	lastEventTime.Store(time.Now().Unix())

	// Subscribe
	reqMsg := map[string]interface{}{
		"kinds": []int{*eventKind},
	}
	reqMsgBytes, _ := json.Marshal(reqMsg)
	subscribeMsg := []interface{}{"REQ", subscriptionID, json.RawMessage(reqMsgBytes)}
	subscribeMsgBytes, _ := json.Marshal(subscribeMsg)

	log.Printf("Sending REQ: %s", string(subscribeMsgBytes))
	if err := conn.WriteMessage(websocket.TextMessage, subscribeMsgBytes); err != nil {
		log.Fatalf("Failed to send REQ: %v", err)
	}

	// Read messages
	gotEOSE := false
	readDone := make(chan struct{})
	consecutiveTimeouts := 0
	maxConsecutiveTimeouts := 20 // Exit if we get too many consecutive timeouts

	go func() {
		defer close(readDone)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				// Check for normal close
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Println("Connection closed normally")
					return
				}

				// Check if context was cancelled
				if ctx.Err() != nil {
					return
				}

				// Check for timeout errors (these are expected during idle periods)
				if netErr, ok := err.(neterr.TimeoutError); ok && netErr.Timeout() {
					consecutiveTimeouts++
					if consecutiveTimeouts >= maxConsecutiveTimeouts {
						log.Printf("Too many consecutive read timeouts (%d), connection may be dead", consecutiveTimeouts)
						return
					}
					// Only log every 5th timeout to avoid spam
					if *verbose && consecutiveTimeouts%5 == 0 {
						log.Printf("Read timeout (idle period, %d consecutive)", consecutiveTimeouts)
					}
					continue
				}

				// For any other error, log and exit
				log.Printf("Read error: %v", err)
				return
			}

			// Reset timeout counter on successful read
			consecutiveTimeouts = 0

			// Parse message
			var envelope []json.RawMessage
			if err := json.Unmarshal(msg, &envelope); err != nil {
				if *verbose {
					log.Printf("Failed to parse message: %v", err)
				}
				continue
			}

			if len(envelope) < 2 {
				continue
			}

			var msgType string
			json.Unmarshal(envelope[0], &msgType)

			// Check message type
			switch msgType {
			case "EOSE":
				var recvSubID string
				json.Unmarshal(envelope[1], &recvSubID)
				if recvSubID == subscriptionID {
					if !gotEOSE {
						gotEOSE = true
						log.Printf("✓ Received EOSE - subscription is active")
						log.Println()
						log.Println("Waiting for real-time events...")
						log.Println()
					}
				}

			case "EVENT":
				var recvSubID string
				json.Unmarshal(envelope[1], &recvSubID)
				if recvSubID == subscriptionID {
					var event NostrEvent
					if err := json.Unmarshal(envelope[2], &event); err == nil {
						count := receivedCount.Add(1)
						lastEventTime.Store(time.Now().Unix())

						eventIDShort := event.ID
						if len(eventIDShort) > 8 {
							eventIDShort = eventIDShort[:8]
						}

						log.Printf("[EVENT #%d] id=%s kind=%d created=%d",
							count, eventIDShort, event.Kind, event.CreatedAt)

						if *verbose {
							log.Printf("  content: %s", event.Content)
						}
					}
				}

			case "NOTICE":
				var notice string
				json.Unmarshal(envelope[1], &notice)
				log.Printf("[NOTICE] %s", notice)

			case "CLOSED":
				var recvSubID, reason string
				json.Unmarshal(envelope[1], &recvSubID)
				if len(envelope) > 2 {
					json.Unmarshal(envelope[2], &reason)
				}
				if recvSubID == subscriptionID {
					log.Printf("⚠ Subscription CLOSED by relay: %s", reason)
					cancel()
					return
				}

			case "OK":
				// Ignore OK messages for this test

			default:
				if *verbose {
					log.Printf("Unknown message type: %s", msgType)
				}
			}
		}
	}()

	// Wait for EOSE with timeout
	eoseTimeout := time.After(10 * time.Second)
	for !gotEOSE {
		select {
		case <-eoseTimeout:
			log.Printf("⚠ Warning: No EOSE received within 10 seconds")
			gotEOSE = true // Continue anyway
		case <-ctx.Done():
			log.Println("Test cancelled before EOSE")
			return
		case <-time.After(100 * time.Millisecond):
			// Keep waiting
		}
	}

	// Monitor for subscription drops
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(*duration) * time.Second)

	// Start monitoring goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(startTime).Seconds()
				lastEvent := lastEventTime.Load()
				timeSinceLastEvent := time.Now().Unix() - lastEvent

				log.Printf("[STATUS] Elapsed: %.0fs/%ds | Events: %d | Last event: %ds ago",
					elapsed, *duration, receivedCount.Load(), timeSinceLastEvent)

				// Warn if no events for a while (but only if we've seen events before)
				if receivedCount.Load() > 0 && timeSinceLastEvent > 30 {
					log.Printf("⚠ Warning: No events received for %ds - subscription may have dropped", timeSinceLastEvent)
				}
			}
		}
	}()

	// Wait for test duration
	log.Printf("Test running for %d seconds...", *duration)
	log.Println("(You can publish events to the relay in another terminal)")
	log.Println()

	select {
	case <-ctx.Done():
		// Test completed or interrupted
	case <-time.After(time.Until(endTime)):
		// Duration elapsed
	}

	// Wait a bit for final events
	time.Sleep(2 * time.Second)
	cancel()

	// Wait for reader to finish
	select {
	case <-readDone:
	case <-time.After(5 * time.Second):
		log.Println("Reader goroutine didn't exit cleanly")
	}

	// Send CLOSE
	closeMsg := []interface{}{"CLOSE", subscriptionID}
	closeMsgBytes, _ := json.Marshal(closeMsg)
	conn.WriteMessage(websocket.TextMessage, closeMsgBytes)

	// Print results
	log.Println()
	log.Println("===================================")
	log.Println("Test Results")
	log.Println("===================================")
	log.Printf("Duration: %.1f seconds", time.Since(startTime).Seconds())
	log.Printf("Events received: %d", receivedCount.Load())
	log.Printf("Subscription ID: %s", subscriptionID)

	lastEvent := lastEventTime.Load()
	if lastEvent > startTime.Unix() {
		log.Printf("Last event: %ds ago", time.Now().Unix()-lastEvent)
	}

	log.Println()

	// Determine pass/fail
	received := receivedCount.Load()
	testDuration := time.Since(startTime).Seconds()

	if received == 0 {
		log.Println("⚠ No events received during test")
		log.Println("This is expected if no events were published")
		log.Println("To test properly, publish events while this is running:")
		log.Println()
		log.Println("  # In another terminal:")
		log.Printf("  ./orly # Make sure relay is running\n")
		log.Println()
		log.Println("  # Then publish test events (implementation-specific)")
	} else {
		eventsPerSecond := float64(received) / testDuration
		log.Printf("Rate: %.2f events/second", eventsPerSecond)

		lastEvent := lastEventTime.Load()
		timeSinceLastEvent := time.Now().Unix() - lastEvent

		if timeSinceLastEvent < 10 {
			log.Println()
			log.Println("✓ TEST PASSED - Subscription remained stable")
			log.Println("Events were received recently, indicating subscription is still active")
		} else {
			log.Println()
			log.Printf("⚠ Potential issue - Last event was %ds ago", timeSinceLastEvent)
			log.Println("Subscription may have dropped if events were still being published")
		}
	}
}
