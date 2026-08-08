package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/zigdon/rsp/cache"
)

type StreamEnvelope struct {
	EventID       string         `json:"id"`
	Category      string         `json:"category"`
	Created       *JSONTime      `json:"created_at"`
	DeviceCode    *CodeAlias     `json:"device_code"`
	DeviceType    string         `json:"device_type"`
	Event         string         `json:"event"`
	Location      LocationID     `json:"location"`
	Payload       map[string]any `json:"payload"`
	ReplicantCode *CodeAlias     `json:"replicant_code"`
	Star          LocationID     `json:"star"`
	Version       int            `json:"version"`
}

func (se *StreamEnvelope) Cache() error {
	if se == nil || db == nil {
		return nil
	}

	data, err := json.Marshal(se.Payload)
	if err != nil {
		return err
	}

	return db.Update(cache.EventsTable, map[string]any{
		"eventid":  se.EventID,
		"category": se.Category,
		"created":  se.Created.Time(),
		"code":     se.DeviceCode.String(),
		"event":    se.Event,
		"location": string(se.Location),
		"data":     data,
	})
}

func (se *StreamEnvelope) Get() error {
	return nil
}

func (se *StreamEnvelope) Header() string {
	return fmt.Sprintf("%s %20s %8s %s",
		se.Created.Time().Format(time.Kitchen), se.Event,
		se.DeviceCode.Alias(), string(se.Location))
}

type StreamAmiDigest struct {
	Directive string     `json:"directive"`
	Report    *AmiReport `json:"report"`
	Activity  struct {
		Event_count int            `json:"event_count"`
		Counts      map[string]int `json:"counts"`
		Window      []*JSONTime    `json:"window"`
	} `json:"activity"`
	Devices []struct {
		DeviceCode *CodeAlias `json:"device_code"`
		Status     string     `json:"status"`
		Events     int        `json:"events"`
		LastEvent  string     `json:"last_event"`
	} `json:"devices"`
}

type AmiReport struct {
	Mining     *AmiMiningReport
	SysSurvey  *AmiSystemSurveyReport
	BeltSurvey *AmiBeltSurveyReport
	Ferry      *AmiFerryReport
	Delivery   *AmiDeliveryReport
}

func (ar *AmiReport) UnmarshalJSON(data []byte) error {
	var pp map[string]any
	if err := json.Unmarshal(data, &pp); err != nil {
		return err
	}
	if _, ok := pp["shortfalls"]; ok {
		return json.Unmarshal(data, &ar.Delivery)
	}
	if _, ok := pp["deliver"]; ok {
		return json.Unmarshal(data, &ar.Ferry)
	}
	if _, ok := pp["belt"]; ok {
		return json.Unmarshal(data, &ar.BeltSurvey)
	}
	if _, ok := pp["scans"]; ok {
		return json.Unmarshal(data, &ar.SysSurvey)
	}
	if _, ok := pp["resources"]; ok {
		return json.Unmarshal(data, &ar.Mining)
	}
	return fmt.Errorf("Can't parse AMI report: %+v", pp)
}

type AmiMiningReport struct {
	Location  LocationID `json:"location"`
	Resources map[string]struct {
		Actual    int  `json:"actual"`
		Capacity  int  `json:"capacity"`
		Desired   int  `json:"desired"`
		Exhausted bool `json:"exhausted"`
	} `json:"resources"`
}

type AmiBeltSurveyReport struct {
	ActiveSites    int    `json:"active_sites"`
	Belt           string `json:"belt"`
	Cruising       int    `json:"cruising"`
	Idle           int    `json:"idle"`
	MaxSites       int    `json:"max_sites"`
	Scans          []any  `json:"scans"`
	Searching      int    `json:"searching"`
	TotalResources int    `json:"total_resources_available"`
}

type AmiSystemSurveyReport struct {
	AssignedThisTick int `json:"assigned_this_tick"`
	Busy             int `json:"busy"`
	Idle             int `json:"idle"`
	Progress         struct {
		Remaining int `json:"remaining"`
		Scanned   int `json:"scanned"`
		Total     int `json:"total"`
	} `json:"progress"`
}

type AmiFerryReport struct {
	CargoCapacity int        `json:"cargo_capacity"`
	CargoCarried  int        `json:"cargo_carried"`
	Collect       LocationID `json:"collect"`
	Deliver       LocationID `json:"deliver"`
	Fleet         struct {
		Delivering int `json:"delivering"`
		Loading    int `json:"loading"`
		Waiting    int `json:"waiting"`
	} `json:"fleet"`
}

type AmiDeliveryReport struct {
	CargoCapacity int        `json:"cargo_capacity"`
	CargoCarried  int        `json:"cargo_carried"`
	Collect       LocationID `json:"collect"`
	Deliver       LocationID `json:"deliver"`
	Fleet         struct {
		Delivering int `json:"delivering"`
		Loading    int `json:"loading"`
		Waiting    int `json:"waiting"`
	} `json:"fleet"`
	Requirement map[string]int `json:"requirement"`
	Shortfalls  map[string]int `json:"shortfalls"`
}

type StreamAmiAdopted struct {
	Devices []*DevicePointer `json:"devices"`
}

type StreamAmiAssembled struct {
	Destination    LocationID `json:"destination"`
	AssembledCount int        `json:"assembled_count"`
}

type StreamAmiLaunched struct {
	DirectiveStatus string `json:"directive_status"`
	Evaluated       bool   `json:"evaluated"`
	DevicesDeployed int    `json:"devices_deployed"`
}

type StreamAmiReleased struct {
	Devices []*DevicePointer `json:"devices"`
}

type StreamAmiWithdrawn struct {
	DirectivePaused bool `json:"directive_paused"`
	DevicesRecalled int  `json:"devices_recalled"`
}

type StreamBlueprintUnlocked struct {
	DeviceType          string         `json:"device_type"`
	ShortDescription    string         `json:"short_description"`
	Description         string         `json:"description"`
	Resources           map[string]int `json:"resources"`
	Components          map[string]int `json:"components"`
	RequiresAutofactory bool           `json:"requires_autofactory"`
}

type StreamBobnetNew struct {
	Id            int        `json:"id"`
	ReplicantName string     `json:"replicant_name"`
	ReplicantCode *CodeAlias `json:"replicant_code"`
	CurrentStar   LocationID `json:"current_star"`
	Channel       string     `json:"channel"`
	Message       string     `json:"message"`
}

type StreamDeviceAttached struct {
	TargetCode *CodeAlias `json:"target_code"`
	TargetType string     `json:"target_type"`
}

type StreamDeviceChanged_owner struct {
	FromReplicant *CodeAlias `json:"from_replicant"`
	ToReplicant   *CodeAlias `json:"to_replicant"`
	Direction     string     `json:"direction"`
}

type StreamDeviceDecommissioned struct {
	ResourcesRecovered  map[string]int `json:"resources_recovered"`
	BlueprintDiscovered string         `json:"blueprint_discovered"`
}

type StreamDeviceDeployed struct {
	DeployedFromDeviceCode *CodeAlias `json:"deployed_from"`
}

type StreamDeviceDetached struct {
	TargetCode *CodeAlias `json:"target_code"`
	TargetType string     `json:"target_type"`
}

type StreamDeviceStowed struct {
	StowedIn *CodeAlias `json:"stowed_in_device_code"`
}

type StreamDirectiveCleared struct {
	PreviousDirective string `json:"previous_directive"`
}

type StreamDirectiveCompleted struct {
	Directive string `json:"directive"`
}

type StreamDirectivePaused struct {
	Directive string `json:"directive"`
}

type StreamDirectiveResumed struct {
	Directive string `json:"directive"`
}

type StreamDirectiveSet struct {
	Directive     string         `json:"directive"`
	Configuration map[string]any `json:"configuration"`
}

type StreamDiversionActivated struct {
	ObjectDesignation LocationID `json:"object_designation"`
	SizeClass         string     `json:"size_class"`
}

type StreamDiversionDeactivated struct {
	DeviceCode *CodeAlias `json:"device_code"`
}

type StreamDiversionDiverted struct {
	ObjectDesignation LocationID `json:"object_designation"`
	Outcome           string     `json:"outcome"`
}

type StreamDiversionImpacted struct {
	ObjectDesignation LocationID `json:"object_designation"`
}

type StreamDiversionPartial struct {
	ObjectDesignation LocationID `json:"object_designation"`
	Outcome           string     `json:"outcome"`
}

type StreamDiversionWear struct {
	OperationalCapacity float32 `json:"operational_capacity"`
	WearAmount          float32 `json:"wear_amount"`
}

type StreamEventCompleted struct {
	Designation string     `json:"designation"`
	Location    LocationID `json:"location"`
	EventType   string     `json:"event_type"`
	Tier        int        `json:"tier"`
	Rewards     struct {
		Xp                 int            `json:"xp"`
		Resources          map[string]int `json:"resources"`
		CivilisationPoints int            `json:"civilisation_points"`
	} `json:"rewards"`
	Consumed struct {
		Devices   []*DevicePointer `json:"devices"`
		Resources map[string]int   `json:"resources"`
	} `json:"consumed"`
}

type StreamEventDiscovered struct {
	Designation string     `json:"designation"`
	Location    LocationID `json:"location"`
	EventType   string     `json:"event_type"`
	Tier        int        `json:"tier"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Criteria    []struct {
		Name      string         `json:"name"`
		Devices   map[string]int `json:"devices"`
		Resources map[string]int `json:"resources"`
	} `json:"criteria"`
}

type StreamExperienceGained struct {
	Source string `json:"source"`
	Amount int    `json:"amount"`
}

type StreamHubActivated struct {
	Star     LocationID `json:"star"`
	Location LocationID `json:"location"`
}

type StreamHubDestroyed struct {
	Star     LocationID `json:"star"`
	Location LocationID `json:"location"`
}

type StreamMegastructureContributed struct {
	MegastructureDesignation string           `json:"megastructure_designation"`
	Accepted_count           int              `json:"accepted_count"`
	ContributedDevices       []*DevicePointer `json:"contributed_devices"`
}

type StreamMessageNew struct {
	MessageId   int    `json:"message_id"`
	MessageType string `json:"message_type"`
	Title       string `json:"title"`
	Body        string `json:"body"`
}

type StreamMiningRetargeted struct {
	LocationType string         `json:"location_type"`
	Location     LocationID     `json:"location"`
	Site         string         `json:"site"`
	OldResource  string         `json:"old_resource"`
	NewResource  string         `json:"new_resource"`
	Availability string         `json:"availability"`
	Density      string         `json:"density"`
	CycleTime    *JSONTimeDelta `json:"cycle_time_seconds"`
}

type StreamMiningStarted struct {
	LocationType string         `json:"location_type"`
	Location     string         `json:"location"`
	Site         string         `json:"site"`
	ResourceType string         `json:"resource_type"`
	Availability string         `json:"availability"`
	Density      string         `json:"density"`
	CycleTime    *JSONTimeDelta `json:"cycle_time_seconds"`
}

type StreamMiningStopped struct {
	Location      LocationID `json:"location"`
	ResourceType  string     `json:"resource_type"`
	QuantityMined int        `json:"quantity_mined"`
}

type StreamPrintStarted struct {
	DeviceType string    `json:"device_type"`
	PrintMode  string    `json:"print_mode"`
	Completes  *JSONTime `json:"completes_at"`
}

type StreamPrintCompleted struct {
	DeviceType    string     `json:"device_type"`
	NewDeviceCode *CodeAlias `json:"new_device_code"`
	PrintMode     string     `json:"print_mode"`
}

type StreamProspectCompleted struct {
	Origin         LocationID   `json:"origin"`
	StarsGenerated int          `json:"stars_generated"`
	Stars          []LocationID `json:"stars"`
}

type StreamRelayActivated struct {
	Star     LocationID `json:"star"`
	Location LocationID `json:"location"`
}

type StreamReplicantTransferred struct {
	OldHost *CodeAlias `json:"old_host"`
	NewHost *CodeAlias `json:"new_host"`
}

type StreamSalvageDepleted struct {
	Site string `json:"site"`
}

type StreamSalvageDiscovered struct {
	Designation string         `json:"designation"`
	Location    LocationID     `json:"location"`
	SalvageType string         `json:"salvage_type"`
	Name        string         `json:"name"`
	Resources   map[string]int `json:"resources"`
}

type StreamScanCompleted struct {
	ScanTarget LocationID `json:"scan_target"`
	ScanType   string     `json:"scan_type"`
	Report     struct {
		Belt struct {
			Density       string     `json:"density"`
			Designation   LocationID `json:"designation"`
			ResourceSites []struct {
				Designation string `json:"designation"`
				Name        string `json:"name"`
				Resources   map[string]struct {
					Availability string `json:"availability"`
					DepletionPct int    `json:"depletion_pct"`
					Original     int    `json:"original"`
					Remaining    int    `json:"remaining"`
				} `json:"resources"`
				SiteIndex int        `json:"site_index"`
				TrackedBy *CodeAlias `json:"tracked_by"`
			} `json:"resource_sites"`
		} `json:"belt"`
	} `json:"report"`
}

type StreamScanStarted struct {
	ScanTarget LocationID     `json:"scan_target"`
	ScanType   string         `json:"scan_type"`
	Eta        *JSONTimeDelta `json:"eta_seconds"`
}

type StreamSearchCompleted struct {
	SearchTarget LocationID `json:"search_target"`
	SearchType   string     `json:"search_type"`
	Report       struct {
		Site      string         `json:"site"`
		Resources map[string]int `json:"resources"`
	} `json:"report"`
}

type StreamSearchStarted struct {
	SearchTarget LocationID     `json:"search_target"`
	SearchType   string         `json:"search_type"`
	Eta          *JSONTimeDelta `json:"eta_seconds"`
}

type StreamSimulationAbandoned struct {
	SimulationId int    `json:"simulation_id"`
	ScenarioCode string `json:"scenario_code"`
}

type StreamSimulationCompleted struct {
	SimulationId   int            `json:"simulation_id"`
	ScenarioCode   string         `json:"scenario_code"`
	Score          *JSONTimeDelta `json:"score_seconds"`
	ResourcesMined int            `json:"resources_mined"`
	DevicesPrinted int            `json:"devices_printed"`
}

type StreamSimulationExpired struct {
	SimulationId int    `json:"simulation_id"`
	ScenarioCode string `json:"scenario_code"`
}

type StreamSimulationStarted struct {
	SimulationId int        `json:"simulation_id"`
	ScenarioCode string     `json:"scenario_code"`
	StartingStar LocationID `json:"starting_star"`
}

type StreamSiteDepleted struct {
	Site string `json:"site"`
}

type StreamStoryAwakened struct {
	NewReplicantCode *CodeAlias `json:"new_replicant_code"`
	NewReplicantName string     `json:"new_replicant_name"`
	HostDeviceCode   *CodeAlias `json:"host_device_code"`
}

type StreamStoryHint struct {
	Hint        string     `json:"hint"`
	Planet      LocationID `json:"planet"`
	Designation string     `json:"designation"`
}

type StreamSystemBody_renamed struct {
	BodyType    string     `json:"body_type"`
	Designation LocationID `json:"designation"`
	NewName     string     `json:"new_name"`
}

type StreamSystemDevices_halted struct {
	Star          LocationID `json:"star"`
	DevicesHalted int        `json:"devices_halted"`
}

type StreamSystemEntry_point_set struct {
	Star       LocationID `json:"star"`
	EntryPoint LocationID `json:"entry_point"`
}

type StreamSystemObject_detected struct {
	ObjectDesignation string     `json:"object_designation"`
	SizeClass         string     `json:"size_class"`
	ImpactTarget      LocationID `json:"impact_target"`
	DiscoverySource   string     `json:"discovery_source"`
}

type StreamTeleportCompleted struct {
	DestinationStar LocationID `json:"destination_star"`
	NewHostCode     *CodeAlias `json:"new_host_code"`
	ReplicantCode   *CodeAlias `json:"replicant_code"`
}

type StreamTeleportFailed struct {
	Reason           string     `json:"reason"`
	TargetMatrixCode *CodeAlias `json:"target_matrix_code"`
}

type StreamTeleportStarted struct {
	SourceStar       LocationID `json:"source_star"`
	DestinationStar  LocationID `json:"destination_star"`
	TargetMatrixCode *CodeAlias `json:"target_matrix_code"`
	ReplicantCode    *CodeAlias `json:"replicant_code"`
}

type StreamTradeCompleted struct {
	TradeCode       string `json:"trade_code"`
	TradeName       string `json:"trade_name"`
	Role            string `json:"role"`
	RemainingStock  int    `json:"remaining_stock"`
	RewardsReceived struct {
		Resources map[string]int `json:"resources"`
		Devices   []*CodeAlias   `json:"devices"`
	} `json:"rewards_received"`
}

type StreamTradeCreated struct {
	TradeCode string `json:"trade_code"`
	Name      string `json:"name"`
	Stock     int    `json:"stock"`
}

type StreamTradeDeleted struct {
	TradeCode      string `json:"trade_code"`
	Name           string `json:"name"`
	RemainingStock int    `json:"remaining_stock"`
}

type StreamTransportCollected struct {
	Resources     map[string]int `json:"resources"`
	Total         int            `json:"total"`
	CargoAfter    int            `json:"cargo_after"`
	CargoCapacity int            `json:"cargo_capacity"`
}

type StreamTransportDelivered struct {
	Resources     map[string]int `json:"resources"`
	Total         int            `json:"total"`
	CargoAfter    int            `json:"cargo_after"`
	CargoCapacity int            `json:"cargo_capacity"`
}

type StreamTravelArrived struct {
	AttachedDevices []*CodeAlias `json:"attached_devices"`
	Destination     LocationID   `json:"destination"`
	Origin          LocationID   `json:"origin"`
	Recalling       bool         `json:"recalling"`
	TravelType      string       `json:"travel_type"`
}

type StreamTravelCancelled struct {
	TravelType      string         `json:"travel_type"`
	Origin          LocationID     `json:"origin"`
	Destination     LocationID     `json:"destination"`
	ReturnTime      *JSONTimeDelta `json:"return_time_seconds"`
	AttachedDevices []*CodeAlias   `json:"attached_devices"`
}

type StreamTravelDeparted struct {
	TravelType      string         `json:"travel_type"`
	Origin          LocationID     `json:"origin"`
	Destination     LocationID     `json:"destination"`
	DistanceAU      float32        `json:"distance_au"`
	DistanceLY      float32        `json:"distance_ly"`
	TravelTime      *JSONTimeDelta `json:"travel_time_seconds"`
	ArrivesAt       *JSONTime      `json:"arrives_at"`
	AttachedDevices []*CodeAlias   `json:"attached_devices"`
	Legs            []struct {
		Type string     `json:"type"`
		To   LocationID `json:"to"`
	} `json:"legs"`
}
