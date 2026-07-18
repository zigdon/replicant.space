package models

import (
	"time"

	"github.com/zigdon/rsp/cache"
)

// Game events
type EventDevice struct {
	Count      int    `json:"count"`
	Current    int    `json:"current"`
	DeviceType string `json:"device_type"`
	Met        bool   `json:"met"`
	Required   int    `json:"required"`
}

type EventCriteria struct {
	Devices   []*EventDevice `json:"devices"`
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
	Devices   []*EventDevice                 `json:"devices"`
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
	Location         LocationID       `json:"location"`
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

type AcceptedContribution struct {
	DeviceCode *CodeAlias `json:"device_code"`
	DeviceType string     `json:"device_type"`
	Value      int        `json:"value"`
}

type RejectedContribution struct {
	DeviceCode *CodeAlias `json:"device_code"`
	Reason     string     `json:"reason"`
}

type Contribute struct {
	Location string                  `json:"location"`
	Accepted []*AcceptedContribution `json:"accepted"`
	Rejected []*RejectedContribution `json:"rejected"`
	Progress struct {
		Percentage float32 `json:"percentage"`
		Stage      string  `json:"stage"`
	} `json:"progress"`
	Status                 string `json:"status"`
	LeaderboardPosition    int    `json:"leaderboard_position"`
	YourTotalContributions int    `json:"your_total_contributions"`
	YourTotalValue         int    `json:"your_total_value"`
}

// Client notifications
type Notification struct {
	ID     int
	Start  time.Time
	End    time.Time
	Device string
	Text   string
	Read   bool
}

func PendingNotifications(read bool) ([]*Notification, error) {
	rows, err := db.PendingNotifications(read)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*Notification
	var ids []int
	for rows.Next() {
		var id int
		var s, e time.Time
		var d, t string
		if err := rows.Scan(&id, &s, &e, &d, &t); err != nil {
			return nil, err
		}
		ids = append(ids, id)
		n := &Notification{
			ID:     id,
			Start:  s,
			End:    e,
			Device: d,
			Text:   t,
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
		"start_ts": n.Start,
		"end_ts":   n.End,
		"device":   n.Device,
		"text":     n.Text,
		"read":     false,
	})
}
