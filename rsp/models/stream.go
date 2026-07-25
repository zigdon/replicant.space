package models

type StreamEnvelope struct {
	Category   string     `json:"category"`
	Created    *JSONTime  `json:"created_at"`
	DeviceCode *CodeAlias `json:"device_code"`
	Star       LocationID `json:"star"`
	Location   LocationID `json:"location"`
	Payload    string     `json:"payload"`
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
	DeployedFromDeviceCode *CodeAlias `json:"deployed_from_device_code"`
}

type StreamDeviceDetached struct {
	TargetCode *CodeAlias `json:"target_code"`
	TargetType string     `json:"target_type"`
}

type StreamDeviceStowed struct {
	StowedInDeviceCode *CodeAlias `json:"stowed_in_device_code"`
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
	DeviceType string `json:"device_type"`
	PrintMode  string `json:"print_mode"`
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
}

type StreamTeleportFailed struct {
	Reason           string     `json:"reason"`
	TargetMatrixCode *CodeAlias `json:"target_matrix_code"`
}

type StreamTeleportStarted struct {
	SourceStar       LocationID `json:"source_star"`
	DestinationStar  LocationID `json:"destination_star"`
	TargetMatrixCode *CodeAlias `json:"target_matrix_code"`
}

type StreamTradeCompleted struct {
	TradeCode      string `json:"trade_code"`
	TradeName      string `json:"trade_name"`
	Role           string `json:"role"`
	RemainingStock int    `json:"remaining_stock"`
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
