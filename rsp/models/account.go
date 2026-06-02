package models

import (
	"fmt"

	"encoding/json"
)

type Notify struct {
	Email       bool            `json:"email"`
	Preferences map[string]bool `json:"preferences"`
	Webhook     bool            `json:"webhook"`
}

type Account struct {
	BobnetChannels        []string    `json:"bobnet_channels"`
	CreatedAt             string      `json:"created_at"`
	Email                 string      `json:"email"`
	EmailVerified         bool        `json:"email_verified"`
	ExperiencePointsTotal int         `json:"experience_points_total"`
	MessageNotify         Notify      `json:"message_notify"`
	Name                  string      `json:"name"`
	Replicants            []Replicant `json:"replicants"`
	Status                string      `json:"status"`
	Timezone              string      `json:"timezone"`
	UnreadMessageCount    int         `json:"unread_message_count"`
}

type Message struct {
	ID        int    `json:"id"`
	Type      string `json:"message_type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"is_read"`
	CreatedAt string `json:"created_at"`
}

type Messages struct {
	Messages    []Message `json:"messages"`
	NextCursor  int       `json:"next_cursor"`
	UnreadCount int       `json:"unread_message_count"`
}

func ParseMessages(data []byte) (*Messages, error) {
	m := &Messages{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("Error parsing messages: %v", err)
	}

	return m, nil
}

func ParseAccount(data []byte) (*Account, error) {
	m := &Account{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("Error parsing me: %v", err)
	}

	return m, nil
}
