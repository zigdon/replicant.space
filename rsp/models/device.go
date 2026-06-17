package models

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type DevicePrint struct {
	CompletesAt     string    `json:"completes_at"`
	Completes       time.Time
	DeviceType      string    `json:"device_type"`
	EtaSeconds      float32   `json:"eta_seconds"`
	Eta             time.Duration
	ProgressPercent float32   `json:"progress_percent"`
	StartedAt       string    `json:"started_at"`
}

func (dp *DevicePrint) Fill() error {
	if err := fillDuration(dp.EtaSeconds, &dp.Eta); err != nil {
		return err
	}
	return fillTime(dp.CompletesAt, &dp.Completes)
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
	EtaSeconds      float32   `json:"eta_seconds"`
	Eta             time.Duration
	ProgressPercent float32   `json:"progress_percent"`
	StartedAt       string    `json:"started_at"`
	Started         time.Time
	Target          string    `json:"target"`
}

func (ds *DeviceScan) Fill() error {
	if err := fillDuration(ds.EtaSeconds, &ds.Eta); err != nil {
		return err
	}
	return fillTime(ds.StartedAt, &ds.Started)
}

type DevicePrintQueue struct {
	Type   string            `json:"device_type"`
	Notify map[string]string `json:"notify"`
}

type Device struct {
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
	ControlledDevices    []*ControlledDevice          `json:"controlled_devices"`
	ControllerDeviceCode *CodeAlias                   `json:"controller_device_code"`
	Features             []string                     `json:"features"`
	InControlRange       bool                         `json:"in_control_range"`
	Location             string                       `json:"location"`
	LocationName         string                       `json:"location_name"`
	OperationalCapacity  float32                      `json:"operational_capacity"`
	Printing             *DevicePrint                 `json:"printing"`
	PrintQueue           []*DevicePrintQueue          `json:"print_queue"`
	QueueSize            int                          `json:"queue_size"`
	ReplicantCode        *CodeAlias                   `json:"replicant_code"`
	Scan                 *DeviceScan                  `json:"scan"`
	Status               string                       `json:"status"`
	StowCapacity         int                          `json:"stow_capacity"`
	StowedDevices        []*StowedDevice              `json:"stowed_devices"`
	StowedInDeviceCode   *CodeAlias                   `json:"stowed_in_device_code"`
	Tags                 []string                     `json:"tags"`
	TaxiMode             string                       `json:"taxi_mode"`
	Travel               *Trip                        `json:"travel"`
	Type                 string                       `json:"device_type"`
	WaitingFor           map[string]*MissingResources `json:"waiting_for"`
}

func (d *Device) Fill() error {
	if d.Printing != nil {
		d.Printing.Fill()
	}
	if d.Travel != nil {
		if err := d.Travel.Fill(); err != nil {
			return err
		}
	}
	return nil
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
	ArrivesAt            string              `json:"arrives_at"`
	Arrives              time.Time
	AssignedDevices      map[string][]string `json:"assigned_devices"`
	AttachedDevices      []string            `json:"attached_devices"`
	AvailableSites       []*AvailableSite    `json:"available_sites"`
	Belt                 string              `json:"belt"`
	CompletesAt          string              `json:"completes_at"`
	Completes            time.Time
	Controller           *ControllerStatus   `json:"controller"`
	ControllerCode       *CodeAlias          `json:"controller_code"`
	DepartedAt           string              `json:"departed_at"`
	Departed             time.Time
	Destination          string              `json:"destination"`
	DestinationName      string              `json:"destination_name"`
	DestinationType      string              `json:"destination_type"`
	DeviceCode           *CodeAlias          `json:"device_code"`
	Eta                  time.Duration
	EtaSeconds           float32         `json:"eta_seconds"`
	FinalDestination     string          `json:"final_destination"`
	FinalDestinationName string          `json:"final_destination_name"`
	JsonErr              string          `json:"error"`
	Location             string          `json:"location"`
	Moon                 *Moon           `json:"moon"`
	Origin               string          `json:"origin"`
	OriginName           string          `json:"origin_name"`
	ProgressPercent      float32         `json:"progress_percent"`
	Released             []*StowedDevice `json:"released"`
	ResourcesRecovered   map[string]int  `json:"resources_recovered"`
	Route                []*TripLeg      `json:"route"`
	Scanned              bool            `json:"scanned"`
	Star                 string          `json:"star"`
	StartedAt            string          `json:"started_at"`
	Started              time.Time
	Status               string          `json:"status"`
	TotalDistanceLy      float32         `json:"total_distance_ly"`
	TotalTime            time.Duration
	TotalTimeSeconds     float32 `json:"total_time_seconds"`
	TravelType           string  `json:"travel_type"`
}

func (cs *CommandResp) Fill() error {
	if err := fillTime(cs.ArrivesAt, &cs.Arrives); err != nil {
		return err
	}
	if err := fillTime(cs.DepartedAt, &cs.Departed); err != nil {
		return err
	}
	if err := fillTime(cs.StartedAt, &cs.Started); err != nil {
		return err
	}
	if err := fillTime(cs.CompletesAt, &cs.Completes); err != nil {
		return err
	}
	if err := fillDuration(cs.EtaSeconds, &cs.Eta); err != nil {
		return err
	}
	if err := fillDuration(cs.TotalTimeSeconds, &cs.TotalTime); err != nil {
		return err
	}
	for _, l := range cs.Route {
		if err := l.Fill(); err != nil {
			return err
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
	Created    time.Time
	CreatedAt  string         `json:"created_at"`
	DeviceCode string         `json:"device_code"`
	DeviceType string         `json:"device_type"`
	EventType  string         `json:"event_type"`
	Id         int            `json:"id"`
	Message    string         `json:"message"`
	Payload    map[string]any `json:"payload"`
}

func (e *DeviceEvent) Fill() error {
	var err error
	e.Created, err = time.Parse(time.RFC3339, e.CreatedAt)
	return err
}

type DeviceLogs struct {
	Events     []*DeviceEvent `json:"events"`
	NextCursor int            `json:"next_cursor"`
}

func (e *DeviceLogs) Fill() error {
	for _, ev := range e.Events {
		if err := ev.Fill(); err != nil {
			return err
		}
	}
	return nil
}
