// Package emailnotify formats a stored notification into an email and publishes
// it to an SNS topic, which fans it out to the subscribed recipient address.
package emailnotify

import (
	"context"
	"encoding/json"
	"fmt"

	"neurosciolar/backend/internal/dynamostore"
)

type Publisher interface {
	Publish(ctx context.Context, subject, message string) error
}

type Notifier struct {
	pub Publisher
}

func New(pub Publisher) *Notifier {
	return &Notifier{pub: pub}
}

func (n *Notifier) Notify(ctx context.Context, note dynamostore.Notification) error {
	subject, body := format(note)
	return n.pub.Publish(ctx, subject, body)
}

func format(note dynamostore.Notification) (subject, body string) {
	var p map[string]any
	_ = json.Unmarshal([]byte(note.Payload), &p)
	switch note.EventType {
	case "ReservationCreated":
		return "Conference Hall - reservation created",
			fmt.Sprintf("Your reservation is confirmed.\n\nHall: %v\nDate: %v %v-%v\nAttendees: %v\nPurpose: %v",
				p["hallId"], p["date"], p["start"], p["end"], p["attendees"], p["purpose"])
	case "FileUploaded":
		return "Conference Hall - file uploaded",
			fmt.Sprintf("Your file was uploaded.\n\nName: %v\nSize: %v bytes", p["originalName"], p["sizeBytes"])
	default:
		return fmt.Sprintf("Conference Hall - %s", note.EventType),
			fmt.Sprintf("Event: %s\n\n%s", note.EventType, note.Payload)
	}
}
