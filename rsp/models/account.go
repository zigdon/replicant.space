package models

import (
	"cmp"
	"slices"
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
	Created               *JSONTime             `json:"created_at"`
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

	UpdateFn func() (*Account, error)
}

func (a *Account) Update() error {
	if a.UpdateFn == nil {
		return nil
	}
	acc, err := a.UpdateFn()
	if err != nil {
		return err
	}
	*a = *acc
	return nil
}

func (a *Account) Fill() error {
	slices.SortFunc(a.ReplicantList, func(a, b *Replicant) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return nil
}

type Message struct {
	ID      int       `json:"id"`
	Type    string    `json:"message_type"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Read    bool      `json:"is_read"`
	Created *JSONTime `json:"created_at"`
}

type Messages struct {
	Messages    []*Message `json:"messages"`
	NextCursor  int        `json:"next_cursor"`
	UnreadCount int        `json:"unread_message_count"`
}

type Bob struct {
	Id            int       `json:"id"`
	Channel       string    `json:"channel"`
	CurrentStar   string    `json:"current_star"`
	Message       string    `json:"message"`
	ReplicantCode string    `json:"replicant_code"`
	ReplicantName string    `json:"replicant_name"`
	Time          *JSONTime `json:"time"`
}

type Bobs struct {
	Messages      []*Bob `json:"messages"`
	NextCursor    int    `json:"next_cursor"`
	Total         int    `json:"total"`
	TotalMessages int    `json:"total_messages"`
}

