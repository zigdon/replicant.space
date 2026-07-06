package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var prospectCmd = &cobra.Command{
	Use:   "prospect",
	Short: "Instruct an observatory to start prospecting for new stars",
	RunE:  prospect,
}

func init() {
	deviceCmd.AddCommand(prospectCmd)
	prospectCmd.Flags().Bool("sol", false, "Prospect towards SOL, for some reason")
	prospectCmd.Flags().StringP("target", "t", "", "Prospect towards this x,y,z coordinate")
}

func prospect(cmd *cobra.Command, args []string) error {
	id, _ := cmd.Flags().GetString("device")
	dev, err := rest.DeviceInfo(models.NewCodeAlias(id))
	if err != nil {
		return fmt.Errorf("Failed to get info for %q: %v", id, err)
	}

	sol, _ := cmd.Flags().GetBool("sol")
	dir, _ := cmd.Flags().GetString("target")
	if !sol && dir == "" {
		log("Starting auto-direction prospect")
		res, err := rest.Prospect(dev.Code, nil)
		if err != nil {
			return err
		}
		prettyPrint(res)
		return nil
	}

	// Get the current location.
	loc := dev.Location
	if strings.Contains(loc, "-") {
		loc = loc[:strings.Index(loc, "-")]
	}
	star, err := rest.Location(loc)
	if err != nil {
		return fmt.Errorf("Can't lookup star %q: %v", loc, err)
	}
	pos := star.Star.Position
	log("Star: %s, Location: %s", loc, pos.String())
	if sol {
		log("Starting prospect towards SOL")
		pos.Reverse()
		res, err := rest.Prospect(dev.Code, pos)
		if err != nil {
			return err
		}
		prettyPrint(res)
		return nil
	}

	target, err := models.ParsePosition(dir)
	if err != nil {
		return fmt.Errorf("Can't parse target %q: %v", dir, err)
	}

	log("Starting prospect towards %s", target.String())
	res, err := rest.Prospect(dev.Code, target.Delta(pos))
	if err != nil {
		return err
	}
	prettyPrint(res)
	return nil
}
