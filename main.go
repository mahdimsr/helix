package main

import (
	"fmt"
	"helix/metatrader"
	"log"
	"net"
	"time"
)

func main() {

	const ADDRESS = "127.0.0.1:8585"

	minutes := 15

	fmt.Printf("Starting Listener...")

	listener, err := net.Listen("tcp", ADDRESS)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Listening to %s", ADDRESS)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		now := time.Now().UTC()
		nextRun := getNextRunMark(now, minutes)
		waitDuration := time.Until(nextRun)

		if err != nil {
			log.Fatalf("Failed to create rabbitmq service: %s", err)
		}

		log.Printf("⌚ Next execution at: %s (waiting %v)", nextRun.Format(time.RFC3339), waitDuration)

		time.Sleep(waitDuration)

		go metatrader.Handle(conn)
	}
}

func getNextRunMark(now time.Time, cycleMinutes int) time.Time {

	nowMinutes := now.Minute()

	nextMinutes := ((nowMinutes / cycleMinutes) + 1) * cycleMinutes

	nextRun := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		nextMinutes,
		0,
		0,
		time.UTC,
	)

	return nextRun
}
