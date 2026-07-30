package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var testCmd = &cobra.Command{
	Use:  "test",
	RunE: readStream,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func readStream(cmd *cobra.Command, args []string) error {
	resources := []string{"carbon", "conductive", "rares", "silicates",
		"structural", "volatiles"}
	type util struct {
		Actual, Capacity int
	}
	log("%20s: %s", "Location", strings.Join(resources, "\t"))

	handle := func(ev map[string]string) error {
		env, err := models.Parse[models.StreamEnvelope]([]byte(ev["data"]))
		if err != nil {
			log("Error parsing %v: %v", ev, err)
			return err
		}
		payload, err := json.Marshal(env.Payload)
		if err != nil {
			log("Can't remarshal payload: %v", err)
			return err
		}
		switch env.Event {
		case "ami.mining.digest":
			ev, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			var data []string
			for _, r := range resources {
				data = append(data,
					fmt.Sprintf("%d/%d", ev.Report.Mining.Resources[r].Actual,
						ev.Report.Mining.Resources[r].Capacity))
			}
			log("%20s: %s", ev.Report.Mining.Location, strings.Join(data, "\t"))

		case "ami.survey.digest", "ami.transport.digest":
			_, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}

		default:
			log("Unknown event type: %q", ev["event"])
			prettyPrint(ev)
		}
		return nil
	}

	if err := rest.ReadStream(handle); err != nil {
		return err
	}

	return nil
}
