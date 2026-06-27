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
		queue := TreeNode("Print Queue")
		for _, pq := range r.PrintQueue {
			queue.AddChild(TreeNode("%s", pq.DeviceType))
		}
		res = append(res, queue)
	}
	if len(r.StowedDevices) > 0 {
		devs := TreeNode("Stowed")
		for _, sd := range r.StowedDevices {
			devs.AddChild(TreeNode("%s: %s", sd.Code.Alias(), sd.Type))
		}
		res = append(res, devs)
	}
	if r.Travel != nil {
		t := r.Travel
		trip := TreeNode("Travel")
		trip.AddChild(TreeNode("From:  %s (%s)", t.Origin, t.Departed.String()))
		trip.AddChild(TreeNode("To:    %s (%s)", t.Destination, t.Arrives.String()))
		trip.AddChild(TreeNode("Stage: %s", t.Stage))
		trip.AddChild(TreeNode("%s", ProgressTime(30, t.Departed.ts, t.Arrives.ts)))
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
