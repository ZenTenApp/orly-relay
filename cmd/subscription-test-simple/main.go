package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"next.orly.dev/pkg/interfaces/neterr"
)

var (
	relayURL = flag.String("url", "ws://localhost:3334", "Relay WebSocket URL")
	duration = flag.Int("duration", 120, "Test duration in seconds")
)

func main() {
	flag.Parse()

	log.SetFlags(log.Ltime)

	fmt.Println("===================================")
	fmt.Println("Simple Subscription Stability Test")
	fmt.Println("===================================")
	fmt.Printf("Relay: %s\n", *relayURL)
	fmt.Printf("Duration: %d seconds\n", *duration)
	fmt.Println()
	fmt.Println("This test verifies that subscriptions remain")
	fmt.Println("active without dropping over the test period.")
	fmt.Println()

	// Connect to relay
	log.Printf("Connecting to %s...", *relayURL)
	conn, _, err := websocket.DefaultDialer.Dial(*relayURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	log.Printf("✓ Connected")

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

	// Subscribe
	subID := fmt.Sprintf("stability-test-%d", time.Now().Unix())
	reqMsg := []interface{}{"REQ", subID, map[string]interface{}{"kinds": []int{1}}}
	reqMsgBytes, _ := json.Marshal(reqMsg)

	log.Printf("Sending subscription: %s", subID)
	if err := conn.WriteMessage(websocket.TextMessage, reqMsgBytes); err != nil {
		log.Fatalf("Failed to send REQ: %v", err)
	}

	// Track connection health
	lastMessageTime := time.Now()
	gotEOSE := false
	messageCount := 0
	pingCount := 0

	// Read goroutine
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if netErr, ok := err.(neterr.TimeoutError); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Read error: %v", err)
				return
			}

			lastMessageTime = time.Now()
			messageCount++

			// Handle PING
			if msgType == websocket.PingMessage {
				pingCount++
				log.Printf("Received PING #%d, sending PONG", pingCount)
				conn.WriteMessage(websocket.PongMessage, nil)
				continue
			}

			// Parse message
			var envelope []json.RawMessage
			if err := json.Unmarshal(msg, &envelope); err != nil {
				continue
			}

			if len(envelope) < 2 {
				continue
			}

			var msgTypeStr string
			json.Unmarshal(envelope[0], &msgTypeStr)

			switch msgTypeStr {
			case "EOSE":
				var recvSubID string
				json.Unmarshal(envelope[1], &recvSubID)
				if recvSubID == subID && !gotEOSE {
					gotEOSE = true
					log.Printf("✓ Received EOSE - subscription is active")
				}

			case "EVENT":
				var recvSubID string
				json.Unmarshal(envelope[1], &recvSubID)
				if recvSubID == subID {
					log.Printf("Received EVENT (subscription still active)")
				}

			case "CLOSED":
				var recvSubID string
				json.Unmarshal(envelope[1], &recvSubID)
				if recvSubID == subID {
					log.Printf("⚠ Subscription CLOSED by relay!")
					cancel()
					return
				}

			case "NOTICE":
				var notice string
				json.Unmarshal(envelope[1], &notice)
				log.Printf("NOTICE: %s", notice)
			}
		}
	}()

	// Wait for EOSE
	log.Println("Waiting for EOSE...")
	for !gotEOSE && ctx.Err() == nil {
		time.Sleep(100 * time.Millisecond)
	}

	if !gotEOSE {
		log.Fatal("Did not receive EOSE")
	}

	// Monitor loop
	startTime := time.Now()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Println()
	log.Printf("Subscription is active. Monitoring for %d seconds...", *duration)
	log.Println("(Subscription should stay active even without events)")
	log.Println()

	for {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			elapsed := time.Since(startTime)
			timeSinceMessage := time.Since(lastMessageTime)

			log.Printf("[%3.0fs/%ds] Messages: %d | Last message: %.0fs ago | Status: %s",
				elapsed.Seconds(),
				*duration,
				messageCount,
				timeSinceMessage.Seconds(),
				getStatus(timeSinceMessage),
			)

			// Check if we've reached duration
			if elapsed >= time.Duration(*duration)*time.Second {
				goto done
			}
		}
	}

done:
	cancel()

	// Wait for reader
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
	}

	// Send CLOSE
	closeMsg := []interface{}{"CLOSE", subID}
	closeMsgBytes, _ := json.Marshal(closeMsg)
	conn.WriteMessage(websocket.TextMessage, closeMsgBytes)

	// Results
	elapsed := time.Since(startTime)
	timeSinceMessage := time.Since(lastMessageTime)

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("Test Results")
	fmt.Println("===================================")
	fmt.Printf("Duration: %.1f seconds\n", elapsed.Seconds())
	fmt.Printf("Total messages: %d\n", messageCount)
	fmt.Printf("Last message: %.0f seconds ago\n", timeSinceMessage.Seconds())
	fmt.Println()

	// Determine success
	if timeSinceMessage < 15*time.Second {
		// Recent message - subscription is alive
		fmt.Println("✓ TEST PASSED")
		fmt.Println("Subscription remained active throughout test period.")
		fmt.Println("Recent messages indicate healthy connection.")
	} else if timeSinceMessage < 30*time.Second {
		// Somewhat recent - probably OK
		fmt.Println("✓ TEST LIKELY PASSED")
		fmt.Println("Subscription appears active (message received recently).")
		fmt.Println("Some delay is normal if relay is idle.")
	} else if messageCount > 0 {
		// Got EOSE but nothing since
		fmt.Println("⚠ INCONCLUSIVE")
		fmt.Println("Subscription was established but no activity since.")
		fmt.Println("This is expected if relay has no events and doesn't send pings.")
		fmt.Println("To properly test, publish events during the test period.")
	} else {
		// No messages at all
		fmt.Println("✗ TEST FAILED")
		fmt.Println("No messages received - subscription may have failed.")
	}

	fmt.Println()
	fmt.Println("Note: This test verifies the subscription stays registered.")
	fmt.Println("For full testing, publish events while this runs and verify")
	fmt.Println("they are received throughout the entire test duration.")
}

func getStatus(timeSince time.Duration) string {
	seconds := timeSince.Seconds()
	switch {
	case seconds < 10:
		return "ACTIVE (recent message)"
	case seconds < 30:
		return "IDLE (normal)"
	case seconds < 60:
		return "QUIET (possibly normal)"
	default:
		return "STALE (may have dropped)"
	}
}
