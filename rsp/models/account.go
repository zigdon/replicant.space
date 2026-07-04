package models

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/zigdon/rsp/cache"
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

func (m *Message) Cache() error {
	return db.Update(cache.MsgTable, map[string]any{
		"id":      m.ID,
		"body":    m.Body,
		"created": m.Created.ts.Unix(),
		"read":    m.Read,
		"type":    m.Type,
		"title":   m.Title,
	})
}

func (m *Message) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if m.ID == 0 {
		return fmt.Errorf("Can't load ID=0")
	}
	scan, err := db.Get(cache.MsgTable, fmt.Sprintf("%d", m.ID))
	if err != nil {
		return err
	}
	var sec int
	err = scan(&m.ID, &m.Body, &sec, &m.Read, &m.Type, &m.Title)
	m.Created = &JSONTime{ts: time.Unix(int64(sec), 0)}
	return err
}

type Messages struct {
	Messages    []*Message `json:"messages"`
	NextCursor  int        `json:"next_cursor"`
	UnreadCount int        `json:"unread_message_count"`
}

func (ms *Messages) Cache() error {
	for _, m := range ms.Messages {
		if err := m.Cache(); err != nil {
			return err
		}
	}
	return nil
}

func (ms *Messages) Get() error {
	for _, m := range ms.Messages {
		if err := m.Get(); err != nil {
			return err
		}
	}
	return nil
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
