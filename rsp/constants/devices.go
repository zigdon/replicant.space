package constants

var RelayTypes = []string{"ftl_relay", "system_hub", "deep_space_relay_station"}
var RelayRanges = map[string]float32{
	"ftl_relay":                7.5,
	"deep_space_relay_station": 10,
	"system_hub":               15,
}
