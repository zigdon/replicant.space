package models

import (
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
	OwnerReplicant       *CodeAlias                   `json:"owner_replicant_code"`
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
			Start: cr.Departed.ts,
			End: cr.Arrives.ts,
			Device: cr.DeviceCode.String(),
			Text: fmt.Sprintf("Arrived at %s", cr.Destination),
		}
	}
	if cr.Started != nil && cr.Completes != nil {
		return &Notification{
			Start: cr.Started.ts,
			End: cr.Completes.ts,
			Device: cr.DeviceCode.String(),
			Text: "Done",
		}
	}
	now := time.Now()
	if cr.Eta != nil {
		return &Notification{
			Start: now,
			End: now.Add(cr.Eta.td),
			Device: cr.DeviceCode.String(),
			Text: "Done",
		}
	}
	if cr.TotalTime != nil {
		return &Notification{
			Start: now,
			End: now.Add(cr.TotalTime.td),
			Device: cr.DeviceCode.String(),
			Text: "Done",
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
