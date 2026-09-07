package constants

import "slices"

var Resources = []string{
	"carbon",
	"conductive",
	"rares",
	"silicates",
	"structural",
	"volatiles",
}

func IsResource(r string) bool {
	return slices.Contains(Resources, r)
}
