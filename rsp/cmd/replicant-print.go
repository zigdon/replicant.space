package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Manage print jobs",
	Long: `By default, start a print of the passed argument.

	To cancel the current job, pass --cancel
	To clear the entire queue, pass --clear`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
		}
		cancel := getBool(cmd, "cancel")
		clear := getBool(cmd, "clear")
		modes := 0
		if cancel {
			modes++
		}
		if clear {
			modes++
		}
		if len(args) > 0 && args[0] != "" {
			modes++
		}
		if modes != 1 {
			return fmt.Errorf("Exactly one of --clear, --cancel, or an argument must be passed.")
		}
		var res *models.PrintResp
		if cancel {
			res, err = rest.ReplicantPrint(rID, "cancel", "")
			if err != nil {
				return fmt.Errorf("Failed to cancel a job: %v", err)
			}
		} else if clear {
			res, err = rest.ReplicantPrint(rID, "clear_queue", "")
			if err != nil {
				return fmt.Errorf("Failed to clear the queue: %v", err)
			}
		} else {
			arg := args[0]
			res, err = rest.ReplicantPrint(rID, "", arg)
			if err != nil {
				return fmt.Errorf("Failed to enqueue %q: %v", arg, err)
			}
		}
		if raw := getBool(cmd, "raw"); raw {
			prettyPrint(res)
			return nil
		}
		printTable([]string{
			"Device Type", "Status", "Started", "Ends", "Duration", "Refunded",
		}, [][]string{{
			res.DeviceType, res.Status, t(res.Started.Time()), t(res.Completes.Time()),
			res.PrintTime.String(), b(res.ResourcesRefunded),
		}})
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(printCmd)
	printCmd.Flags().Bool("cancel", false, "Cancel the current print job")
	printCmd.Flags().Bool("clear", false, "Clear the entire print queue")
}
