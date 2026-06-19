package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// " huh, so each site keeps survey/mining controllers, and drones, so keep mining forever?"

// Automatically set up persistent belt mining site
// ami mining + mining drone
// ami scanning + scanning drone
// ftl relay
// Tag with mine-SYSTEM-BELT-1
// Build missing devices
// Deliver built devices
// Adopt drones to ami
// Set ami policy

// Collect resources
// Pick site outside of the allowlist that has the most resources
// Set ami transport to ferry stuff home

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "High level automation commands",
}

var autoMineCmd = &cobra.Command{
	Use:   "mine",
	Short: "Set up a new belt mine",
	RunE:  autoMine,
}

func init() {
	rootCmd.AddCommand(autoCmd)

	autoCmd.PersistentFlags().Bool("dry_run", true, "When set, only describe what will be done")
	autoCmd.PersistentFlags().String("owner", "zigdon-2", "Replicant responsible for printing new devices")

	autoCmd.AddCommand(autoMineCmd)
	autoMineCmd.Flags().StringP("location", "l", "", "Belt location to mine")
	autoMineCmd.MarkFlagRequired("location")
	autoMineCmd.Flags().StringSliceP("factory", "f", []string{"a-1"}, "Devices for building new ships")
}

func autoMine(cmd *cobra.Command, args []string) error {
	// Validate the location
	locName, _ := cmd.Flags().GetString("location")
	loc, err := rest.Location(locName)
	if err != nil {
		return err
	}

	// Get the existing fleet
	tag := fmt.Sprintf("mine-%s", loc.Location)
	devs, err := rest.GetTagged(tag)

	// Build whatever is missing
	missing := map[string]int{
		"ami_mining_controller": 1,
		"ami_survey_controller": 1,
		"maintenance_drone":     1,
		"mining_drone":          3,
		"survey_drone":          2,
		"ftl_relay":             1,
	}
	var data [][]string
	fleet := make(map[string][]*models.Device)
	for k, v := range missing {
		data = append(data, []string{k, d(v), "", ""})
	}

	amis := make(map[string]string)
	for _, d := range devs.Devices {
		t := d.Type
		if strings.Contains(t, "ami") {
			amis[t] = d.Code.String()
		}
		missing[t] -= 1
		fleet[t] = append(fleet[t], d)
	}
	for _, l := range data {
		var f []string
		for _, d := range fleet[l[0]] {
			f = append(f, alias(d.Code.String()))
		}
		slices.Sort(f)
		l[2] = list(f)
		l[3] = d(missing[l[0]])
	}
	printTable([]string{"Device", "Target", "Found", "Missing"}, data)

	// Enqueue a build
	var printing bool
	printers, _ := cmd.Flags().GetStringSlice("factory")
	for devType, qty := range missing {
		if qty <= 0 {
			continue
		}
		factory, err := findPrinter(printers)
		if err != nil {
			return fmt.Errorf("No available factory found to queue %s: %v", devType, err)
		}
		cfg := map[string]any{
			"device_type": devType,
		}
		if t, ok := strings.CutSuffix(devType, "_drone"); ok {
			if c, ok := amis[fmt.Sprintf("ami_%s_controller", t)]; ok {
				cfg["controller"] = c
			}
		}
		fmt.Printf("Printing %q at %q...\n", devType, factory)
		res, err := rest.DeviceCommand(factory, "enqueue_print", cfg)
		if err != nil {
			return err
		}
		prettyPrint(res)
		printing = true
	}

	if printing {
		fmt.Println("Waiting for missing devices.")
		return nil
	}

	fmt.Println("Fleet ready to ship")

	return nil
}

func findPrinter(printers []string) (string, error) {
	// Check the queue for each potential printer. If there is an idle printer,
	// use that. Otherwise, pick the one with the shortest queue, by remaining
	// print time.
	info := make(map[string]*models.Device)
	fmt.Println("Printers:")
	for _, p := range printers {
		i, err := rest.DeviceInfo(p)
		if err != nil {
			return "", fmt.Errorf("can't get device info for %q: %v", p, err)
		}
		info[p] = i
		fmt.Printf("  %s: %s\n", p, i.Type)
	}

	// Calculate the queue length for each printer
	queue := make(map[string]time.Duration)
	for _, p := range printers {
		eta, err := rest.GetPrintQueueETA(info[p])
		if err != nil {
			return "", fmt.Errorf("error getting print queue for %q: %v", p, err)
		}
		queue[p] = eta
	}
	if len(queue) == 0 {
		return "", fmt.Errorf("No available printer found")
	}
	slices.SortFunc(printers, func(a, b string) int {
		ta, _ := queue[a]
		tb, _ := queue[b]
		return cmp.Compare(ta, tb)
	})

	return printers[0], nil
}
