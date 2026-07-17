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

var rootPrintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the current queue of all home autofactories",
	RunE:  rootPrintList,
}

func init() {
	rootCmd.AddCommand(rootPrintCmd)
	rootPrintCmd.Flags().String("home", "MENKUNT-2-L4", "Where can autofactories be found")
	rootPrintCmd.Flags().IntP("repeat", "r", 1, "How many copies should be printed")
	rootPrintCmd.Flags().StringP("controller", "c", "", "What controller should be assigned")
	rootPrintCmd.Flags().String("on_complete", "", "What commands to execute once done")

	rootPrintCmd.AddCommand(rootPrintListCmd)
	rootPrintListCmd.Flags().String("home", "MENKUNT-2-L4", "Where can autofactories be found")
}

func getHomeFactories(home string) ([]*models.CodeAlias, error) {
	factories, err := rest.Devices(map[string]string{"location": home, "device_type": "autofactory"})
	if err != nil {
		return nil, err
	}
	if len(factories) == 0 {
		return nil, fmt.Errorf("No factories found at %s", home)
	}
	log("%d factories found", len(factories))
	var printers []*models.CodeAlias
	for _, f := range factories {
		printers = append(printers, f.Code)
	}
	return printers, nil
}

func rootPrintList(cmd *cobra.Command, args []string) error {
	home, _ := cmd.Flags().GetString("home")
	printers, err := getHomeFactories(home)
	if err != nil {
		return err
	}
	type pq struct {
		code       *models.CodeAlias
		deviceType string
		tags       []string
		pos        int
		eta        time.Time
	}
	times := make(map[*models.CodeAlias]time.Duration)
	var queue []pq
	for _, p := range printers {
		info, err := getInfo(p)
		if err != nil {
			return err
		}
		if string(info.Location) != home {
			continue
		}
		if info.Status == "waiting_for_resources" {
			queue = append(queue, pq{
				code:       p,
				deviceType: "Waiting for resources",
				tags: []string{
					v(info.WaitingFor),
				},
				pos: -1,
			})
		}
		if info.Printing != nil {
			queue = append(queue, pq{
				code:       p,
				deviceType: info.Printing.DeviceType,
				tags:       info.Printing.Tags,
				pos:        -1,
				eta:        info.Printing.Completes.Time(),
			})
			times[p] += info.Printing.Eta.Duration()
		}
		for i, q := range info.PrintQueue {
			bp := getBP(q.Type)
			queue = append(queue, pq{
				code:       p,
				deviceType: q.Type,
				tags:       q.Tags,
				pos:        i,
				eta:        time.Now().Add(bp.PrintTime.Duration()).Add(times[p]),
			})
		}
	}

	slices.SortFunc(queue, func(a, b pq) int {
		return cmp.Compare(a.eta.Unix(), b.eta.Unix())
	})
	var data [][]string
	for _, q := range queue {
		var pos string
		if q.pos < 0 {
			pos = "printing"
		} else {
			pos = d(q.pos)
		}
		data = append(data, []string{
			q.deviceType, list(q.tags), q.code.Alias(), pos, t(q.eta),
		})
	}
	printTable([]string{"Type", "Tags", "Factory", "Position", "ETA"}, data)

	return nil
}

func rootPrint(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("Usage: rsp print <device> [-r <copies>]")
	}
	bp := getBP(args[0])
	log("Print time, per copy: %s", bp.PrintTime.Duration())

	home, _ := cmd.Flags().GetString("home")
	printers, err := getHomeFactories(home)
	if err != nil {
		return err
	}

	copies, _ := cmd.Flags().GetInt("repeat")
	controller, _ := cmd.Flags().GetString("controller")
	onComplete, _ := cmd.Flags().GetString("on_complete")
	queue := make(map[string]time.Duration)
	added := make(map[string]int)
	cfg := map[string]any{
		"device_type": bp.DeviceType,
	}
	if controller != "" {
		cfg["controller"] = controller
	}
	if onComplete != "" {
		cfg["oncomplete"] = onComplete
	}

	for ; copies > 0; copies-- {
		p, err := rest.FindPrinter(printers, queue)
		if err != nil {
			return err
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
		var q, eta []string
		if dev, err := getInfo(p); err == nil {
			if dev.Printing != nil {
				q = append(q, dev.Printing.DeviceType)
				eta = append(eta, dev.Printing.Eta.String())
			}
			for _, pq := range dev.PrintQueue {
				q = append(q, pq.Type)
				eta = append(eta, getBP(pq.Type).PrintTime.String())
			}
			for i := 0; i < added[p.Alias()]; i++ {
				q = append(q, bp.DeviceType)
				eta = append(eta, bp.PrintTime.String())
			}
		}
		data = append(data, []string{p.Alias(), d(added[p.Alias()]), lines(q), lines(eta)})
	}
	printTable([]string{"Factory", "Copies", "Queue", "ETA"}, data)
	return nil
}
