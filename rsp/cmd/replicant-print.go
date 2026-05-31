package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Queue a print job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("Device type must be passed as an argument")
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
		res, err := rest.Print(rID, args[0])
		if err != nil {
			return fmt.Errorf("Failed to enqueue %q: %v", args[0], err)
		}
		// if raw, _ := cmd.Flags().GetBool("raw"); raw { prettyPrint(rep) }
		prettyPrint(res)
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(printCmd)
}
