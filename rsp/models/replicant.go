package models

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/rivo/tview"
)

type ReplicantEvent struct {
	Created    *JSONTime      `json:"created_at"`
	DeviceCode *CodeAlias     `json:"device_code"`
	DeviceType string         `json:"device_type"`
	Type       string         `json:"event_type"`
	Message    string         `json:"message"`
	Payload    map[string]any `json:"payload"`
}

type ReplicantEvents struct {
	ReplicantEvents []*ReplicantEvent `json:"events"`
}

type OwnedDevices struct {
	Devices []*Device `json:"devices"`
}

type PrintQueue struct {
	DeviceType string `json:"device_type"`
	Notify     struct {
		Device  string `json:"device"`
		Email   bool   `json:"email"`
		Webhook bool   `json:"webhook"`
	} `json:"notify"`
}

type MissingResources struct {
	Have int `json:"have"`
	Need int `json:"need"`
}

type Teleport struct {
	Status           string         `json:"status"`
	SourceStar       string         `json:"source_star"`
	DestinationStar  string         `json:"destination_star"`
	Started          *JSONTime      `json:"started_at"`
	Completes        *JSONTime      `json:"completes_at"`
	Offline          *JSONTimeDelta `json:"offline_seconds"`
	TargetMatrixCode *CodeAlias     `json:"target_matrix_code"`
	Error            string         `json:"error"`
}

func (t *Teleport) Notification() *Notification {
	return &Notification{
		Start:  t.Started.ts,
		End:    t.Completes.ts.Add(t.Offline.td),
		Device: "Replicant",
		Text:   fmt.Sprintf("Online in %s at %s", t.TargetMatrixCode.Alias(), t.DestinationStar),
	}
}

type Travel struct {
	Arrives         *JSONTime      `json:"arrives_at"`
	Departed        *JSONTime      `json:"departed_at"`
	Destination     string         `json:"destination"`
	DestinationName string         `json:"destination_name"`
	DestinationType string         `json:"destination_type"`
	Eta             *JSONTimeDelta `json:"eta_seconds"`
	Origin          string         `json:"origin"`
	OriginName      string         `json:"origin_name"`
	ProgressPercent float32        `json:"progress_percent"`
	Route           []*TripLeg     `json:"route"`
	Stage           string         `json:"stage"`
	TotalDistanceLy float32        `json:"total_distance_ly"`
	TotalTime       *JSONTimeDelta `json:"total_time_seconds"`
	Type            string         `json:"type"`
}

func (t *Travel) Notification() *Notification {
	return &Notification{
		Start:  t.Departed.ts,
		End:    t.Arrives.ts,
		Device: "Replicant",
		Text:   fmt.Sprintf("Arrived at %s", t.Destination),
	}
}

type Replicant struct {
	AttachedDevices     []string                     `json:"attached_devices"`
	Cargo               []*Inventory                 `json:"cargo"`
	Code                *CodeAlias                   `json:"replicant_code"`
	Created             *JSONTime                    `json:"created_at"`
	CurrentLocation     string                       `json:"current_location"`
	CurrentLocationName string                       `json:"current_location_name"`
	CurrentStar         string                       `json:"current_star"`
	CurrentStarName     string                       `json:"current_star_name"`
	Description         string                       `json:"description"`
	DeviceCount         int                          `json:"device_count"`
	ExperiencePoints    int                          `json:"experience_points"`
	HostedDeviceCode    *CodeAlias                   `json:"hosted_device_code"`
	IsNPC               bool                         `json:"is_npc"`
	Location            string                       `json:"location"`
	LocationName        string                       `json:"location_name"`
	Name                string                       `json:"name"`
	Plan                string                       `json:"plan"`
	Position            *Position                    `json:"position"`
	Printing            *DevicePrint                 `json:"printing"`
	PrintQueue          []*PrintQueue                `json:"print_queue"`
	Project             string                       `json:"project"`
	Pronouns            string                       `json:"pronouns"`
	Status              string                       `json:"status"`
	StowedDevices       []*Device                    `json:"stowed_devices"`
	Travel              *Travel                      `json:"travel"`
	WaitingFor          map[string]*MissingResources `json:"waiting_for"`

	UpdateFn func(*CodeAlias) (*Replicant, error)
}

func (r *Replicant) Update() error {
	if r.UpdateFn == nil {
		return nil
	}
	nr, err := r.UpdateFn(r.Code)
	if err != nil {
		return err
	}
	*r = *nr

	return nil
}

func (r *Replicant) ID() *CodeAlias {
	return r.Code
}

func (r *Replicant) Type() string {
	return "replicant"
}

func (r *Replicant) Fill() error {
	slices.SortFunc(r.Cargo, func(a, b *Inventory) int {
		return cmp.Compare(a.ResourceType, b.ResourceType)
	})
	slices.SortFunc(r.StowedDevices, func(a, b *Device) int {
		return cmp.Or(
			cmp.Compare(a.Type, b.Type),
			cmp.Compare(a.Code.Alias(), b.Code.Alias()),
		)
	})
	slices.Sort(r.AttachedDevices)
	return nil
}

func (r *Replicant) Details() []*tview.TreeNode {
	res := []*tview.TreeNode{TreeNode("%s", r.Name).
		AddChild(TreeNode("%s: %s", r.Code.Alias(), r.Code.String())).
		AddChild(TreeNodeFn("Location: %s", ref(r.CurrentLocation))).
		AddChild(TreeNodeFn("Status: %s", ref(r.Status))).
		AddChild(TreeNodeFn("XP: %d", ref(r.ExperiencePoints)))}
	if len(r.Cargo) > 0 {
		cargo := TreeNodeGen("Cargo", func() (res []string) {
			for _, c := range r.Cargo {
				res = append(res, fmt.Sprintf("%3.0f x %s", c.Quantity, c.ResourceType))
			}
			return
		})
		res = append(res, cargo)
	}
	if len(r.AttachedDevices) > 0 {
		ad := TreeNodeGen("Attached Devices", func() (res []string) {
			for _, d := range r.AttachedDevices {
				res = append(res, d)
			}
			return
		})
		res = append(res, ad)
	}
	if len(r.PrintQueue) > 0 {
		queue := TreeNodeGen("Print Queue", func() (res []string) {
			for _, pq := range r.PrintQueue {
				res = append(res, pq.DeviceType)
			}
			return
		})
		res = append(res, queue)
	}
	if len(r.StowedDevices) > 0 {
		devs := TreeNodeGen("Stowed", func() (res []string) {
			for _, sd := range r.StowedDevices {
				res = append(res, fmt.Sprintf("%s: %s", sd.Code.Alias(), sd.Type))
			}
			return
		})
		res = append(res, devs)
	}
	if r.Travel != nil {
		t := r.Travel
		trip := TreeNodeGen("Travel", func() []string {
			return []string{
				fmt.Sprintf("From:  %s (%s)", t.Origin, t.Departed.String()),
				fmt.Sprintf("To:    %s (%s)", t.Destination, t.Arrives.String()),
				fmt.Sprintf("Stage: %s", t.Stage),
				fmt.Sprintf("%s", ProgressTime(30, t.Departed.ts, t.Arrives.ts))}
		})
		res = append(res, trip)
	}

	return res
}

func (r *Replicant) GetDeviceIDs() []string {
	var res []string
	for _, d := range r.StowedDevices {
		res = append(res, d.Code.String())
	}
	return res
}

func (r *Replicant) ListItem() (string, string) {
	return fmt.Sprintf("%s: %s", r.Code.Alias(), r.Name), r.CurrentLocation
}
