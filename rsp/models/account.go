package models

type Notify struct {
	Email       bool                       `json:"email"`
	Preferences map[string]map[string]bool `json:"preferences"`
	Webhook     bool                       `json:"webhook"`
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

type Bob struct {
	Id            int    `json:"id"`
	Channel       string `json:"channel"`
	CurrentStar   string `json:"current_star"`
	Message       string `json:"message"`
	ReplicantCode string `json:"replicant_code"`
	ReplicantName string `json:"replicant_name"`
	Time          string `json:"time"`
}

type Bobs struct {
	Messages      []Bob `json:"messages"`
	NextCursor    int   `json:"next_cursor"`
	Total         int   `json:"total"`
	TotalMessages int   `json:"total_messages"`
}
