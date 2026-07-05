package models

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"
)

type DevicePrint struct {
	Completes       *JSONTime      `json:"completes_at"`
	DeviceType      string         `json:"device_type"`
	Eta             *JSONTimeDelta `json:"eta_seconds"`
	ProgressPercent float32        `json:"progress_percent"`
	Started         *JSONTime      `json:"started_at"`
}

type ControlledDevice struct {
	Code     *CodeAlias `json:"device_code"`
	Type     string     `json:"device_type"`
	Location string     `json:"location"`
	Status   string     `json:"status"`
}

type StowedDevice struct {
	Code *CodeAlias `json:"device_code"`
	Type string     `json:"device_type"`
}

type DeviceDirective struct {
	Config map[string]any `json:"config"`
	Name   string         `json:"name"`
}

type DeviceScan struct {
	Eta             *JSONTimeDelta `json:"eta_seconds"`
	ProgressPercent float32        `json:"progress_percent"`
	Started         *JSONTime      `json:"started_at"`
	Target          string         `json:"target"`
}

type DevicePrintQueue struct {
	Controller string            `json:"Controller"`
	Type       string            `json:"device_type"`
	Notify     map[string]string `json:"notify"`
}

type Inventory struct {
	Quantity     float32 `json:"quantity"`
	ResourceType string  `json:"resource_type"`
}

func (i *Inventory) Short() string {
	return fmt.Sprintf("%.0f x %s", i.Quantity, i.ResourceType)
}

type SystemStatus struct {
	Active           bool    `json:"active"`
	ActiveDiversions int     `json:"active_diversions"`
	HubCapacity      float32 `json:"hub_capacity"`
	Star             string  `json:"star"`
}

type UpkeepRequirement struct {
	QuantityPer20pct int    `json:"quantity_per_20pct"`
	ResourceType     string `json:"resource_type"`
}

func (ur *UpkeepRequirement) String() string {
	return fmt.Sprintf("%d x %s", ur.QuantityPer20pct, ur.ResourceType)
}

type Repair struct {
	Eta              *JSONTimeDelta `json:"eta_seconds"`
	ProgressPercent  float32        `json:"progress_percent"`
	Started          *JSONTime      `json:"started_at"`
	TargetDeviceCode *CodeAlias     `json:"target_device_code"`
}

func SortDevices(ds []*Device) {
	slices.SortFunc(ds, func(a, b *Device) int {
		return cmp.Or(
			cmp.Compare(a.Code.Type(), b.Code.Type()),
			cmp.Compare(a.Code.Num(), b.Code.Num()),
		)
	})
}

type Compact struct {
	Completes       JSONTime      `json:"completes_at"`
	Eta             JSONTimeDelta `json:"eta_seconds"`
	ProgressPercent float32       `json:"progress_percent"`
	Started         JSONTime      `json:"started_at"`
}

type Device struct {
	fetchedAt time.Time

	AmiDirective         *DeviceDirective             `json:"ami_directive"`
	AmiDirectiveStatus   string                       `json:"ami_directive_status"`
	AttachCapacity       int                          `json:"attach_capacity"`
	AttachedDevices      []*Device                    `json:"attached_devices"`
	AttachedToDeviceCode *CodeAlias                   `json:"attached_to_device_code"`
	AvailableCommands    []string                     `json:"available_commands"`
	AvailableDirectives  []string                     `json:"available_directives"`
	Cargo                []*Inventory                 `json:"cargo"`
	CargoCapacity        int                          `json:"cargo_capacity"`
	Code                 *CodeAlias                   `json:"device_code"`
	Compact              *Compact                     `json:"compact"`
	ControlledDevices    []*ControlledDevice          `json:"controlled_devices"`
	ControllerDeviceCode *CodeAlias                   `json:"controller_device_code"`
	Created              *JSONTime                    `json:"created_at"`
	Deployed             *JSONTime                    `json:"deployed_at"`
	Features             []string                     `json:"features"`
	GracePeriodRemaining int                          `json:"grace_period_remaining"`
	InControlRange       bool                         `json:"in_control_range"`
	Location             string                       `json:"location"`
	LocationName         string                       `json:"location_name"`
	OperationalCapacity  float32                      `json:"operational_capacity"`
	OwnerReplicant       *CodeAlias                   `json:"owner_replicant_code"`
	PrintQueue           []*DevicePrintQueue          `json:"print_queue"`
	Printing             *DevicePrint                 `json:"printing"`
	QueueSize            int                          `json:"queue_size"`
	Repair               *Repair                      `json:"repair"`
	RepairPaidPct        float32                      `json:"repair_paid_pct"`
	ReplicantCode        *CodeAlias                   `json:"replicant_code"`
	Scan                 *DeviceScan                  `json:"scan"`
	Status               string                       `json:"status"`
	StowCapacity         int                          `json:"stow_capacity"`
	StowUsed             int                          `json:"stow_used"`
	StowedDevices        []*StowedDevice              `json:"stowed_devices"`
	StowedInDeviceCode   *CodeAlias                   `json:"stowed_in_device_code"`
	SystemStatus         *SystemStatus                `json:"system_status"`
	Tags                 []string                     `json:"tags"`
	TaxiMode             string                       `json:"taxi_mode"`
	Travel               *Trip                        `json:"travel"`
	Type                 string                       `json:"device_type"`
	UpkeepRequirements   []*UpkeepRequirement         `json:"upkeep_requirements"`
	WaitingFor           map[string]*MissingResources `json:"waiting_for"`
	WelcomeMessage       string                       `json:"welcome_message"`
}

func (d *Device) Alias() {
	if db != nil && d.Code.Alias() == d.Code.String() {
		if a, err := db.Alias(d.Code.String(), d.Type); err == nil {
			d.Code.alias = a
		}
	}
}

func (d *Device) Fetched() time.Time {
	return d.fetchedAt
}

func (d *Device) Fill() error {
	d.fetchedAt = time.Now()
	if strings.Contains(d.Status, "repairing (") {
		target := d.Status[strings.Index(d.Status, "(")+1 : strings.Index(d.Status, ")")]
		s, err := db.Alias(target, d.Type)
		if err != nil {
			return err
		}
		d.Status = fmt.Sprintf("repairing (%s)", s)
	}
	return nil
}

func (d *Device) GetPosition() *Position {
	if db == nil || d.Location == "" {
		return nil
	}
	loc := d.Location
	if i := strings.Index(d.Location, "-"); i > 0 {
		loc = d.Location[:i]
	}
	s := &Star{Designation: loc}
	if err := s.Get(); err != nil {
		return nil
	}
	return s.Position
}

type ControllerStatus struct {
	DirectivePaused       bool   `json:"directive_paused"`
	DirectiveResumed      bool   `json:"directive_resumed"`
	DirectiveStatusAfter  string `json:"directive_status_after"`
	DirectiveStatusBefore string `json:"directive_status_before"`
	RecallResult          string `json:"recall_result"`
}

type CommandResp struct {
	AdoptedDevices       []*StowedDevice     `json:"adopted"`
	AmiDirective         *DeviceDirective    `json:"ami_directive"`
	AmiDirectiveStatus   string              `json:"ami_directive_status"`
	Arrives              *JSONTime           `json:"arrives_at"`
	AssignedDevices      map[string][]string `json:"assigned_devices"`
	Attached             []*StowedDevice     `json:"attached"`
	AttachedDevices      []string            `json:"attached_devices"`
	AvailableSites       []*AvailableSite    `json:"available_sites"`
	Belt                 string              `json:"belt"`
	BlueprintDiscovered  string              `json:"blueprint_discovered"`
	CarrierCode          *CodeAlias          `json:"carrier_code"`
	Completes            *JSONTime           `json:"completes_at"`
	Controller           *ControllerStatus   `json:"controller"`
	ControllerCode       *CodeAlias          `json:"controller_code"`
	Departed             *JSONTime           `json:"departed_at"`
	Destination          string              `json:"destination"`
	DestinationName      string              `json:"destination_name"`
	DestinationType      string              `json:"destination_type"`
	Detached             []*StowedDevice     `json:"detached"`
	DeviceCode           *CodeAlias          `json:"device_code"`
	Eta                  *JSONTimeDelta      `json:"eta_seconds"`
	FinalDestination     string              `json:"final_destination"`
	FinalDestinationName string              `json:"final_destination_name"`
	Location             string              `json:"location"`
	Moon                 *Moon               `json:"moon"`
	Origin               string              `json:"origin"`
	OriginName           string              `json:"origin_name"`
	ProgressPercent      float32             `json:"progress_percent"`
	Queue                []*DevicePrintQueue `json:"queue"`
	QueueLength          int                 `json:"queue_length"`
	Released             []*StowedDevice     `json:"released"`
	ResourcesRecovered   map[string]int      `json:"resources_recovered"`
	Route                []*TripLeg          `json:"route"`
	Scanned              bool                `json:"scanned"`
	Star                 string              `json:"star"`
	Started              *JSONTime           `json:"started_at"`
	Status               string              `json:"status"`
	TotalDistanceLy      float32             `json:"total_distance_ly"`
	TotalTime            *JSONTimeDelta      `json:"total_time_seconds"`
	TravelType           string              `json:"travel_type"`
	StowedIn             *CodeAlias          `json:"stowed_in"`
}

func (cr *CommandResp) Notification() *Notification {
	if cr.Departed != nil && cr.Arrives != nil {
		return &Notification{
			Start:  cr.Departed.ts,
			End:    cr.Arrives.ts,
			Device: cr.DeviceCode.String(),
			Text:   fmt.Sprintf("Arrived at %s", cr.Destination),
		}
	}
	if cr.Started != nil && cr.Completes != nil {
		return &Notification{
			Start:  cr.Started.ts,
			End:    cr.Completes.ts,
			Device: cr.DeviceCode.String(),
			Text:   "Done",
		}
	}
	now := time.Now()
	if cr.Eta != nil {
		return &Notification{
			Start:  now,
			End:    now.Add(cr.Eta.td),
			Device: cr.DeviceCode.String(),
			Text:   "Done",
		}
	}
	if cr.TotalTime != nil {
		return &Notification{
			Start:  now,
			End:    now.Add(cr.TotalTime.td),
			Device: cr.DeviceCode.String(),
			Text:   "Done",
		}
	}
	return nil
}

type AvailableSite struct {
	Designation string `json:"designation"`
	Name        string `json:"name"`
	SalvageType string `json:"salvage_type"`
}

type TaggedDevices struct {
	Devices    []*Device `json:"devices"`
	NextCursor int       `json:"next_cursor"`
}

func (td *TaggedDevices) Fill() error {
	for _, d := range td.Devices {
		if err := d.Fill(); err != nil {
			return err
		}
	}
	return nil
}

type NetworkNode struct {
	DeviceCode *CodeAlias `json:"device_code"`
	DistanceLy float32    `json:"distance_ly"`
	Star       string     `json:"star"`
}

type Network struct {
	Connections []*NetworkNode `json:"connections"`
	Error       string         `json:"error"`
	RangeLy     float32        `json:"range_ly"`
	Status      string         `json:"status"`
}

func (n *Network) Fill() error {
	slices.SortFunc(n.Connections, func(a, b *NetworkNode) int {
		return cmp.Compare(a.Star, b.Star)
	})
	return nil
}

func (n *Network) Devices() []string {
	var res []string
	for _, nn := range n.Connections {
		res = append(res, nn.DeviceCode.String())
	}
	return res
}

func (n *Network) Stars() []string {
	var res []string
	for _, nn := range n.Connections {
		res = append(res, nn.Star)
	}
	return res
}

func (n *Network) String() string {
	var res []string
	for _, nn := range n.Connections {
		res = append(res, fmt.Sprintf("%s (%s)", nn.Star, nn.DeviceCode.alias))
	}
	slices.Sort(res)
	return strings.Join(res, ", ")
}

func (n *Network) Equal(n2 *Network) bool {
	if len(n2.Connections) == 0 {
		return false
	}
	return slices.Contains(n.Stars(), n2.Connections[0].Star)
}

type DeviceEvent struct {
	Created    *JSONTime      `json:"created_at"`
	DeviceCode string         `json:"device_code"`
	DeviceType string         `json:"device_type"`
	EventType  string         `json:"event_type"`
	Id         int            `json:"id"`
	Message    string         `json:"message"`
	Payload    map[string]any `json:"payload"`
}

type DeviceLogs struct {
	Events     []*DeviceEvent `json:"events"`
	NextCursor int            `json:"next_cursor"`
}
