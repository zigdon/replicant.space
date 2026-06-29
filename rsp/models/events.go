package models

import (
	"time"

	"github.com/zigdon/rsp/cache"
)

// Game events
type EventCriteria struct {
	Devices   []any          `json:"devices"`
	Name      string         `json:"name"`
	Resources map[string]int `json:"resources"`
}

type EventProgressResourceOption struct {
	Current      float32 `json:"current"`
	Met          bool    `json:"met"`
	Required     int     `json:"required"`
	ResourceType string  `json:"resource_type"`
}

type EventProgressOption struct {
	Devices   []any                          `json:"devices"`
	Met       bool                           `json:"met"`
	Name      string                         `json:"name"`
	Resources []*EventProgressResourceOption `json:"resources"`
}

type EventProgress struct {
	Met              bool                   `json:"met"`
	MetOption        string                 `json:"met_option"`
	Options          []*EventProgressOption `json:"options"`
	ReplicantPresent bool                   `json:"replicant_present"`
}

type EventReward struct {
	CivilisationPoints    int            `json:"civilisation_points"`
	CompletionAchievement string         `json:"completion_achievement"`
	Resources             map[string]int `json:"resources"`
	XP                    int            `json:"xp"`
}

type Event struct {
	BroadcastMessage string           `json:"broadcast_message"`
	Category         string           `json:"category"`
	Criteria         []*EventCriteria `json:"criteria"`
	Description      string           `json:"description"`
	Designation      string           `json:"designation"`
	Discovered       *JSONTime        `json:"discovered_at"`
	Error            string           `json:"error"`
	Location         string           `json:"location"`
	LocationName     string           `json:"location_name"`
	Progress         *EventProgress   `json:"progress"`
	Rewards          *EventReward     `json:"rewards"`
	Status           string           `json:"status"`
	Tier             int              `json:"tier"`
	Title            string           `json:"title"`
	Type             string           `json:"event_type"`
}

type Events struct {
	Events     []*Event `json:"events"`
	NextCursor int      `json:"next_cursor"`
}

// Client notifications
type Notification struct {
	ID int
	Start time.Time
	End time.Time
	Device string
	Text string
}

func PendingNotifications() ([]*Notification, error) {
	rows, err := db.PendingNotifications()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*Notification
	var ids []int
	for rows.Next() {
		var id int
		var s, e int64
		var d, t string
		if err := rows.Scan(&id, &s, &e, &d, &t); err != nil {
			return nil, err
		}
		ids = append(ids, id)
		n := &Notification{
			ID: id,
			Start: time.Unix(s, 0).Local(),
			End: time.Unix(e, 0).Local(),
			Device: d,
			Text: t,
		}
		res = append(res, n)
	}

	return res, db.ClearNotifications(ids)
}

func (n *Notification) Save() error {
	if n == nil {
		return nil
	}
	return db.Update(cache.NotificationTable, map[string]any{
		"start": n.Start.Second(),
		"end": n.End.Second(),
		"device": n.Device,
		"text": n.Text,
	})
}
