package models

import (
	"time"
)

type NotifyDetails struct {
	Email   map[string]bool `json:"email"`
	Webhook map[string]bool `json:"webhook"`
}

type Notify struct {
	Email       bool           `json:"email"`
	Preferences *NotifyDetails `json:"preferences"`
	Webhook     bool           `json:"webhook"`
}

type AccountUpdate struct {
	BobnetChannels       []string `json:"bobnet_channels,omitempty"`
	MessageNotify        *Notify  `json:"message_notify,omitempty"`
	Name                 string   `json:"name,omitempty"`
	Timezone             string   `json:"timezone,omitempty"`
	ReplicantCooperation string   `json:"replicant_cooperation,omitempty"`
}

type Account struct {
	BobnetChannels        []string              `json:"bobnet_channels"`
	CreatedAt             string                `json:"created_at"`
	Created               time.Time
	Email                 string                `json:"email"`
	EmailVerified         bool                  `json:"email_verified"`
	ExperiencePointsTotal int                   `json:"experience_points_total"`
	MessageNotify         *Notify               `json:"message_notify"`
	Name                  string                `json:"name"`
	ReplicantCooperation  string                `json:"replicant_cooperation"`
	ReplicantList         []*Replicant          `json:"replicants"`
	Replicants            map[string]*Replicant `json:"-"`
	Status                string                `json:"status"`
	Timezone              string                `json:"timezone"`
	UnreadMessageCount    int                   `json:"unread_message_count"`
}

func (a *Account) Fill() error {
	return fillTime(a.CreatedAt, &a.Created)
}

type Message struct {
	ID        int    `json:"id"`
	Type      string `json:"message_type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"is_read"`
	Created   time.Time
	CreatedAt string `json:"created_at"`
}

func (m *Message) Fill() error {
	return fillTime(m.CreatedAt, &m.Created)
}

type Messages struct {
	Messages    []*Message `json:"messages"`
	NextCursor  int        `json:"next_cursor"`
	UnreadCount int        `json:"unread_message_count"`
}

func (m *Messages) Fill() error {
	return fill([]fillData{{recurse: f(m.Messages)}})
}

type Bob struct {
	Id            int       `json:"id"`
	Channel       string    `json:"channel"`
	CurrentStar   string    `json:"current_star"`
	Message       string    `json:"message"`
	ReplicantCode string    `json:"replicant_code"`
	ReplicantName string    `json:"replicant_name"`
	TimeRaw       string    `json:"time"`
	Time 		  time.Time
}

func (b *Bob) Fill() error {
	return fillTime(b.TimeRaw, &b.Time)
}

type Bobs struct {
	Messages      []*Bob `json:"messages"`
	NextCursor    int    `json:"next_cursor"`
	Total         int    `json:"total"`
	TotalMessages int    `json:"total_messages"`
}

func (bs *Bobs) Fill() error {
	return fill([]fillData{{recurse: f(bs.Messages)}})
}

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
	Discovered       time.Time
	DiscoveredAt     string           `json:"discovered_at"`
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

func (e *Event) Fill() error {
	return fillTime(e.DiscoveredAt, &e.Discovered)
}

type Events struct {
	Events     []*Event `json:"events"`
	NextCursor int      `json:"next_cursor"`
}

func (es *Events) Fill() error {
	return fill([]fillData{{recurse: f(es.Events)}})
}
