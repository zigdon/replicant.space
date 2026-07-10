package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var rootPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "Queue a print job at an autofactory",
	RunE:  rootPrint,
}

func init() {
	rootCmd.AddCommand(rootPrintCmd)
	rootPrintCmd.Flags().String("home", "MENKUNT-BELT-1", "Where can autofactories be found")
	rootPrintCmd.Flags().IntP("repeat", "r", 1, "How many copies should be printed")
}

func rootPrint(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("Usage: rsp print <device> [-r <copies>]")
	}
	bp := &models.Blueprint{DeviceType: args[0]}
	if err := bp.Get(); err != nil {
		return fmt.Errorf("Can load blueprint for %s: %v", args[0], err)
	}
	log("Print time, per copy: %s", bp.PrintTime.Duration())

	home, _ := cmd.Flags().GetString("home")
	factories, err := rest.Devices(map[string]string{"location": home, "device_type": "autofactory"})
	if err != nil {
		return err
	}
	if len(factories) == 0 {
		return fmt.Errorf("No factories found at %s", home)
	}
	log("%d factories found", len(factories))
	var printers []*models.CodeAlias
	for _, f := range factories {
		printers = append(printers, f.Code)
	}

	copies, _ := cmd.Flags().GetInt("repeat")
	controller, _ := cmd.Flags().GetString("controller")
	onComplete, _ := cmd.Flags().GetString("on_complete")
	queue := make(map[string]time.Duration)
	added := make(map[string]int)
	for ; copies > 0; copies-- {
		p, err := rest.FindPrinter(printers, queue)
		if err != nil {
			return err
		}
		cfg := map[string]any{
			"device_type": bp.DeviceType,
		}
		if controller != "" {
			cfg["controller"] = controller
		}
		if onComplete != "" {
			cfg["oncomplete"] = onComplete
		}
		_, err = rest.DeviceCommand[models.CommandResp](p, "enqueue_print", cfg)
		if err != nil {
			return err
		}
		added[p.Alias()]++
		queue[p.String()] += bp.PrintTime.Duration()
	}
	slices.SortFunc(printers, func(a, b *models.CodeAlias) int {
		return cmp.Compare(a.Num(), b.Num())
	})
	var data [][]string
	for _, p := range printers {
		if added[p.Alias()] == 0 {
			continue
		}
		data = append(data, []string{p.Alias(), d(added[p.Alias()])})
	}
	printTable([]string{"Factory", "Copies"}, data)
	return nil
}
