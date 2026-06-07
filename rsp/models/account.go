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
	MessageNotify         *Notify      `json:"message_notify"`
	Name                  string      `json:"name"`
	ReplicantList         []*Replicant `json:"replicants"`
	Replicants            map[string]*Replicant
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
	Messages    []*Message `json:"messages"`
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
	Messages      []*Bob `json:"messages"`
	NextCursor    int   `json:"next_cursor"`
	Total         int   `json:"total"`
	TotalMessages int   `json:"total_messages"`
}

type EventCriteria struct {
	Devices []any `json:"devices"`
	Name string `json:"name"`
	Resources map[string]int `json:"resources"`
}

type EventProgressResourceOption struct {
	Current float32 `json:"current"`
	Met bool `json:"met"`
	Required int `json:"required"`
	ResourceType string `json:"resource_type"`
}

type EventProgressOption struct {
	Devices []any `json:"devices"`
	Met bool `json:"met"`
	Name string `json:"name"`
	Resources []*EventProgressResourceOption `json:"resources"`
}

type EventProgress struct {
	Met bool `json:"met"`
	MetOption string `json:"met_option"`
	Options []*EventProgressOption `json:"options"`
	ReplicantPresent bool `json:"replicant_present"`
}

type EventReward struct {
	CivilisationPoints int `json:"civilisation_points"`
	CompletionAchievement string `json:"completion_achievement"`
	Resources map[string]int `json:"resources"`
	XP int `json:"xp"`
}

type Event struct {
	BroadcastMessage string `json:"broadcast_message"`
	Category string `json:"category"`
	Criteria []*EventCriteria `json:"criteria"`
	Description string `json:"description"`
	Designation string `json:"designation"`
	DiscoveredAt string `json:"discovered_at"`
	Location string `json:"location"`
	LocationName string `json:"location_name"`
	Progress *EventProgress `json:"progress"`
	Rewards *EventReward `json:"rewards"`
	Status string `json:"status"`
	Tier int `json:"tier"`
	Title string `json:"title"`
	Type string `json:"event_type"`
}

type Events struct {
	Events []*Event `json:"events"`
	NextCursor int `json:"next_cursor"`
}
