package cmd

import (
	"encoding/json"
	"fmt"

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
	handle := func(ev map[string]string) error {
		fmt.Println()
		env, err := models.Parse[models.StreamEnvelope]([]byte(ev["data"]))
		if err != nil {
			log("Error parsing %v: %v", ev, err)
			return err
		}
		log(env.Header())
		payload, err := json.Marshal(env.Payload)
		if err != nil {
			log("Can't remarshal payload: %v", err)
			return err
		}
		switch env.Event {
		case "ami.mining.digest", "ami.survey.digest", "ami.transport.digest":
			ev, err := models.Parse[models.StreamAmiDigest](payload)
			if err != nil {
				log("%s parse error: %v", env.Event, err)
				return err
			}
			log("%v", ev)
			printTable(ev.Columns(), ev.Lines())

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
