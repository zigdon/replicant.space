package cmd

import "github.com/spf13/cobra"

// Check if the destination already has a working relay
// - If yes, check that it's in the home network
// - If it isn't, plot the relay path from home, and rerun with the next step
//   that is missing a relay.
// If there wasn't a working relay, find an idle one (or print one)
// Transport it to the destination system
// Activate it

func autoRelay(cmd *cobra.Command, args []string) error {
	// locName, _ := cmd.Flags().GetString("location")
	return nil
}
