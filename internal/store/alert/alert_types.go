package alert

import (
	"errors"
	"time"

	"dash/internal/model"
)

var ErrAlertRuleVersionStale = errors.New("alert rule version stale")

type AlertOpenEventParams struct {
	RuleID             int64
	RuleGeneration     int64
	Builtin            bool
	RuleSnapshot       []byte
	ObjectType         model.ObjectType
	ObjectID           int64
	TriggeredAt        time.Time
	CurrentValue       float64
	EffectiveThreshold float64
	Title              string
	Message            string
	Notifications      []AlertNotificationParams
}

type AlertOpenEventResult struct {
	EventID int64
	Created bool
}

type AlertNotificationPayload struct {
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type AlertNotificationParams struct {
	Transition  string
	ChannelID   int64
	ChannelType model.NotifyType
	Payload     AlertNotificationPayload
}

type CloseStatus string

const (
	CloseStatusClosed   CloseStatus = "closed"
	CloseStatusNotFound CloseStatus = "not_found"
)

type AlertCloseEventParams struct {
	EventID        int64
	RuleID         int64
	RuleGeneration int64
	ObjectType     model.ObjectType
	ObjectID       int64
	ClosedAt       time.Time
	CurrentValue   *float64
	CloseReason    string
	Notifications  []AlertNotificationParams
}

type AlertCloseEventResult struct {
	EventID int64
	Status  CloseStatus
}
