package models

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/zigdon/rsp/cache"
)

type DevicePrint struct {
	Completes       *JSONTime      `json:"completes_at"`
	DeviceType      string         `json:"device_type"`
	Eta             *JSONTimeDelta `json:"eta_seconds"`
	ProgressPercent float32        `json:"progress_percent"`
	Started         *JSONTime      `json:"started_at"`
	Tags            []string       `json:"tags"`
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

type StowedDevices struct {
	Devices []*StowedDevice `json:"devices"`
}

func (sd *StowedDevices) UnmarshalJSON(data []byte) error {
	// First try to parse it as the list of structs
	if err := json.Unmarshal(data, &sd.Devices); err == nil {
		return nil
	}
	// Failing that, try just a list of strings
	var ids []string
	sd.Devices = make([]*StowedDevice, 0)
	if err := json.Unmarshal(data, &ids); err != nil {
		return err
	}
	for _, id := range ids {
		sd.Devices = append(sd.Devices, &StowedDevice{Code: NewCodeAlias(id)})
	}
	return nil
}

func (sd *StowedDevices) MarshalJSON() ([]byte, error) {
	return json.Marshal(sd.Devices)
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
	Tags       []string          `json:"tags"`
}

type Inventory struct {
	Quantity     float32 `json:"quantity"`
	ResourceType string  `json:"resource_type"`
}

func (i *Inventory) String() string {
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

type DeviceUpdate struct {
	AmiDirective         *DeviceDirective    `json:"ami_directive"`
	AmiDirectiveStatus   string              `json:"ami_directive_status"`
	AttachedToDeviceCode *CodeAlias          `json:"attached_to_device_code"`
	AvailableCommands    []string            `json:"available_commands"`
	AvailableDirectives  []string            `json:"available_directives"`
	Code                 *CodeAlias          `json:"device_code"`
	Compact              *Compact            `json:"compact"`
	ControllerDeviceCode *CodeAlias          `json:"controller_device_code"`
	Created              *JSONTime           `json:"created_at"`
	Features             []string            `json:"features"`
	InControlRange       bool                `json:"in_control_range"`
	Location             LocationID          `json:"location"`
	LocationName         string              `json:"location_name"`
	OperationalCapacity  float32             `json:"operational_capacity"`
	PrintQueue           []*DevicePrintQueue `json:"print_queue"`
	Printing             *DevicePrint        `json:"printing"`
	Prospect             *Prospect           `json:"prospect"`
	QueueSize            int                 `json:"queue_size"`
	Repair               *Repair             `json:"repair"`
	ReplicantCode        *CodeAlias          `json:"replicant_code"`
	Scan                 *DeviceScan         `json:"scan"`
	Status               string              `json:"status"`
	StowedInDeviceCode   *CodeAlias          `json:"stowed_in_device_code"`
	Tags                 []string            `json:"tags"`
	Travel               *Trip               `json:"travel"`
	Type                 string              `json:"device_type"`
	Unfurl               *Compact            `json:"unfurl"`
}

func (pd *DeviceUpdate) Apply() *Device {
	// Grab a cached version of the device, update the fields we fetched
	d := &Device{Code: pd.Code}
	// Try to fetch the saved data, if we have any. If not, fill in the
	// immutable fields too.
	if err := d.Get(); err != nil {
		d.AvailableCommands = pd.AvailableCommands
		d.AvailableDirectives = pd.AvailableDirectives
		d.Created = pd.Created
		d.Type = pd.Type
		d.Features = pd.Features
	}
	d.fetchedAt.update = time.Now()
	d.AmiDirective = pd.AmiDirective
	d.AmiDirectiveStatus = pd.AmiDirectiveStatus
	d.AttachedToDeviceCode = pd.AttachedToDeviceCode
	d.Compact = pd.Compact
	d.ControllerDeviceCode = pd.ControllerDeviceCode
	d.InControlRange = pd.InControlRange
	d.Location = pd.Location
	d.LocationName = pd.LocationName
	d.OperationalCapacity = pd.OperationalCapacity
	d.PrintQueue = pd.PrintQueue
	d.Printing = pd.Printing
	d.Prospect = pd.Prospect
	d.QueueSize = pd.QueueSize
	d.Repair = pd.Repair
	d.ReplicantCode = pd.ReplicantCode
	d.Scan = pd.Scan
	d.Status = pd.Status
	d.StowedInDeviceCode = pd.StowedInDeviceCode
	d.Tags = pd.Tags
	d.Travel = pd.Travel
	d.Unfurl = pd.Unfurl
	d.Cache()
	return d
}

type Device struct {
	fetchedAt struct {
		full   time.Time
		update time.Time
	}

	AmiDirective         *DeviceDirective    `json:"ami_directive"`
	AmiDirectiveStatus   string              `json:"ami_directive_status"`
	AttachCapacity       int                 `json:"attach_capacity"`
	AttachedDevices      []*Device           `json:"attached_devices"`
	AttachedToDeviceCode *CodeAlias          `json:"attached_to_device_code"`
	AvailableCommands    []string            `json:"available_commands"`
	AvailableDirectives  []string            `json:"available_directives"`
	Cargo                []*Inventory        `json:"cargo"`
	CargoCapacity        int                 `json:"cargo_capacity"`
	Code                 *CodeAlias          `json:"device_code"`
	Compact              *Compact            `json:"compact"`
	ControlledDevices    []*ControlledDevice `json:"controlled_devices"`
	ControllerDeviceCode *CodeAlias          `json:"controller_device_code"`
	Created              *JSONTime           `json:"created_at"`
	Deployed             *JSONTime           `json:"deployed_at"`
	Features             []string            `json:"features"`
	GracePeriodRemaining int                 `json:"grace_period_remaining"`
	InControlRange       bool                `json:"in_control_range"`
	Location             LocationID          `json:"location"`
	LocationName         string              `json:"location_name"`
	OperationalCapacity  float32             `json:"operational_capacity"`
	Owner                *struct {
		Name string     `json:"name"`
		Code *CodeAlias `json:"replicant_code"`
	} `json:"owner"`
	OwnerReplicant     *CodeAlias           `json:"owner_replicant_code"`
	PrintQueue         []*DevicePrintQueue  `json:"print_queue"`
	Printing           *DevicePrint         `json:"printing"`
	Prospect           *Prospect            `json:"prospect"`
	QueueSize          int                  `json:"queue_size"`
	Repair             *Repair              `json:"repair"`
	RepairPaidPct      float32              `json:"repair_paid_pct"`
	ReplicantCode      *CodeAlias           `json:"replicant_code"`
	Scan               *DeviceScan          `json:"scan"`
	Status             string               `json:"status"`
	StowCapacity       int                  `json:"stow_capacity"`
	StowUsed           int                  `json:"stow_used"`
	StowedDevices      *StowedDevices       `json:"stowed_devices"`
	StowedInDeviceCode *CodeAlias           `json:"stowed_in_device_code"`
	SystemStatus       *SystemStatus        `json:"system_status"`
	Tags               []string             `json:"tags"`
	TaxiMode           string               `json:"taxi_mode"`
	Travel             *Trip                `json:"travel"`
	Type               string               `json:"device_type"`
	Unfurl             *Compact             `json:"unfurl"`
	UpkeepRequirements []*UpkeepRequirement `json:"upkeep_requirements"`
	WaitingFor         struct {
		Components map[string]*MissingResources `json:"components"`
		Resources  map[string]*MissingResources `json:"resources"`
	} `json:"waiting_for"`
	WelcomeMessage string `json:"welcome_message"`
}

func (d *Device) Alias() {
	if db != nil && d.Code.Alias() == d.Code.String() {
		if a, err := db.Alias(d.Code.String(), d.Type); err == nil {
			d.Code.alias = a
		}
	}
}

func (d *Device) Fetched() time.Time {
	return d.fetchedAt.full
}

func (d *Device) SetFetched() {
	if d == nil {
		return
	}
	d.fetchedAt.full = time.Now()
	d.Cache()
}

func (d *Device) Updated() time.Time {
	return d.fetchedAt.update
}

func (d *Device) Fill() error {
	if strings.Contains(d.Status, "repairing (") {
		target := d.Status[strings.Index(d.Status, "(")+1 : strings.Index(d.Status, ")")]
		s, err := db.Alias(target, d.Type)
		if err != nil {
			return err
		}
		d.Status = fmt.Sprintf("repairing (%s)", s)
	}
	if d.Travel != nil {
		d.Travel.Device = d.Code
	}
	return nil
}

func (d *Device) GetPosition() *Position {
	if db == nil || d.Location == "" {
		return nil
	}
	s, err := NewStar(string(d.Location.Star()))
	if err != nil {
		return nil
	}
	return s.Position
}

func (d *Device) Cache() error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	err = db.Update(cache.JSONDevices, map[string]any{
		"code":       d.Code.String(),
		"fetched_ts": d.Fetched(),
		"updated_ts": d.Updated(),
		"location":   d.Location,
		"type":       d.Type,
		"data":       data,
	})
	return err
}

func (d *Device) Get() error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}
	if d.Code == nil {
		return fmt.Errorf("Can't load unknown device")
	}

	row := db.DB.QueryRow(`
		SELECT fetched_ts, updated_ts, data
		FROM json_devices
		WHERE code = $1`, d.Code.String())
	if err := row.Err(); err != nil {
		return err
	}
	var data []byte
	if err := row.Scan(&d.fetchedAt.full, &d.fetchedAt.update, &data); err != nil {
		return err
	}

	pd, err := Parse[Device](data)
	pd.fetchedAt = d.fetchedAt
	*d = *pd
	return err
}

type ControllerStatus struct {
	DirectivePaused       bool   `json:"directive_paused"`
	DirectiveResumed      bool   `json:"directive_resumed"`
	DirectiveStatusAfter  string `json:"directive_status_after"`
	DirectiveStatusBefore string `json:"directive_status_before"`
	RecallResult          string `json:"recall_result"`
}

type CommandResp struct {
	AdoptedDevices       *StowedDevices      `json:"adopted"`
	AmiDirective         *DeviceDirective    `json:"ami_directive"`
	AmiDirectiveStatus   string              `json:"ami_directive_status"`
	Arrives              *JSONTime           `json:"arrives_at"`
	AssignedDevices      map[string][]string `json:"assigned_devices"`
	Attached             *StowedDevices      `json:"attached"`
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
	Detached             *StowedDevices      `json:"detached"`
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
	Released             *StowedDevices      `json:"released"`
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
			Object: cr,
		}
	}
	if cr.Started != nil && cr.Completes != nil {
		return &Notification{
			Start:  cr.Started.ts,
			End:    cr.Completes.ts,
			Device: cr.DeviceCode.String(),
			Text:   "Done",
			Object: cr,
		}
	}
	now := time.Now()
	if cr.Eta != nil {
		return &Notification{
			Start:  now,
			End:    now.Add(cr.Eta.td),
			Device: cr.DeviceCode.String(),
			Text:   "Done",
			Object: cr,
		}
	}
	if cr.TotalTime != nil {
		return &Notification{
			Start:  now,
			End:    now.Add(cr.TotalTime.td),
			Device: cr.DeviceCode.String(),
			Text:   "Done",
			Object: cr,
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
	Updates    []*DeviceUpdate `json:"devices"`
	NextCursor int             `json:"next_cursor"`
	Devices    []*Device
}

func (td *TaggedDevices) Fill() error {
	for _, u := range td.Updates {
		d := u.Apply()
		if err := d.Fill(); err != nil {
			return err
		}
		td.Devices = append(td.Devices, d)
	}
	return nil
}

func (td *TaggedDevices) Get() error {
	// Not implemented
	return nil
}

func (td *TaggedDevices) Cache() error {
	var errs []error
	for _, d := range td.Devices {
		errs = append(errs, d.Cache())
	}
	return errors.Join(errs...)
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

type ProspectDetail struct {
	Expected          float32 `json:"expected"`
	Neighbours        int     `json:"neighbours"`
	OutwardNeighbours int     `json:"outward_neighbours"`
	OutwardRatio      float32 `json:"outward_ratio"`
	Ratio             float32 `json:"ratio"`
}

type Prospect struct {
	Completes       JSONTime        `json:"completes_at"`
	Detail          *ProspectDetail `json:"detail"`
	Error           string          `json:"error"`
	Eta             JSONTimeDelta   `json:"eta_seconds"`
	ProgressPercent float32         `json:"progress_percent"`
	Started         JSONTime        `json:"started_at"`
}
