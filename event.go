package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/getsentry/sentry-go"
)

type TypeSwitch struct {
	Kind string `json:"type"`
}

type Event struct {
	TypeSwitch `json:"type"`
	*Error
	*Transaction
	*DSN
}

func (event *Event) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &event.TypeSwitch); err != nil {
		return err
	}
	switch event.Kind {
	case ERROR:
		event.Error = &Error{}
		return json.Unmarshal(data, event.Error)
	case TRANSACTION:
		event.Transaction = &Transaction{}
		return json.Unmarshal(data, event.Transaction)
	default:
		sentry.CaptureMessage("unrecognized type value " + event.Kind)
		return fmt.Errorf("unrecognized type value %q", event.Kind)
	}
}

func (event *Event) setDsn() {
	if event.Kind == TRANSACTION && event.Transaction.Platform == JAVASCRIPT {
		event.DSN = NewDSN(os.Getenv("DSN_JAVASCRIPT_SAAS"))
	}
	if event.Kind == TRANSACTION && event.Transaction.Platform == PYTHON {
		event.DSN = NewDSN(os.Getenv("DSN_PYTHON_SAAS"))
	}
	if event.Kind == ERROR && event.Error.Platform == JAVASCRIPT {
		event.DSN = NewDSN(os.Getenv("DSN_JAVASCRIPT_SAAS"))
	}
	if event.Kind == ERROR && event.Error.Platform == PYTHON {
		event.DSN = NewDSN(os.Getenv("DSN_PYTHON_SAAS"))
	}
}