package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Manage print jobs",
	Long: `By default, start a print of the passed argument.

	To cancel the current job, pass --cancel
	To clear the entire queue, pass --clear`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cancel, _ := cmd.Flags().GetBool("cancel")
		clear, _ := cmd.Flags().GetBool("clear")
		modes := 0
		if cancel { modes ++ }
		if clear { modes ++ }
		if len(args) > 0 && args[0] != "" { modes ++ }
		if modes != 1 {
			return fmt.Errorf("Exactly one of --clear, --cancel, or an argument must be passed.")
		}
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				return fmt.Errorf("Replicant #%d not found: %v", id, err)
			}
			rID = code
		}
		var res *models.PrintResp
		var err error
		if cancel {
			res, err = rest.PrintCmd(rID, "cancel")
			if err != nil {
				return fmt.Errorf("Failed to cancel a job: %v", err)
			}
		} else if clear {
			res, err = rest.PrintCmd(rID, "clear_queue")
			if err != nil {
				return fmt.Errorf("Failed to clear the queue: %v", err)
			}
		} else {
			arg := args[0]
			res, err = rest.Print(rID, arg)
			if err != nil {
				return fmt.Errorf("Failed to enqueue %q: %v", arg, err)
			}
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(res)
		} else {
			printTable([]string{
				"Device Type", "Status", "Started", "Ends", "Duration", "Refunded",
			}, [][]string{{
				res.DeviceType, res.Status, res.StartedAt, res.CompletesAt,
				res.PrintTimeSeconds.String(), b(res.ResourcesRefunded),
			}}, 0)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(printCmd)
	printCmd.Flags().Bool("cancel", false, "Cancel the current print job")
	printCmd.Flags().Bool("clear", false, "Clear the entire print queue")
}
