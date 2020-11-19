package main

import (
	"flag"
	"log"

	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var (
	all      *bool
	ignore   *bool
	DSNs     map[string]*DSN
	traceIds []string
)

func init() {

	if err := godotenv.Load(); err != nil {
		sentry.CaptureMessage("No .env file found")
		log.Print("No .env file found")
	}

	ignore = flag.Bool("i", false, "ignore sending the event to Sentry.io")
	flag.Parse()
	initializeSentry()

}

func main() {
	sentry.CaptureMessage("job started")

	demoAutomation := DemoAutomation{}

	events := demoAutomation.getEvents()

	for _, event := range events {
		if event.Kind == ERROR {
			event.Error.eventId()
			event.Error.release()
			event.Error.user()
			event.Error.timestamp()
		}
		if event.Kind == "transaction" {
			event.Transaction.eventId()
			event.Transaction.release()
			event.Transaction.user()
			event.Transaction.timestamps()
		}
	}

	getTraceIds(events)
	updateTraceIds(events)

	requests := Requests{events}
	requests.send()
}