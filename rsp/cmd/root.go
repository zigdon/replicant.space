package cmd

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var db *cache.Cache

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rsp",
	Short: "Simple cli for interacting with replicant.space",
}

// Execute adds all child commands to the root command and sets flags
// appropriately.  This is called by main.main(). It only needs to happen once
// to the rootCmd.
func Execute() {
	// Connect to the database
	var err error
	db, err = cache.Connect()
	if err != nil {
		log("Failed to connect to db: %v", err)
	} else {
		common.ConnectDB(db)
		models.ConnectDB(db)
		rest.ConnectDB(db)
	}

	err = rootCmd.Execute()
	if err != nil {
		die(err.Error())
	}
	if msg := getBool(rootCmd, "msg"); !slices.Contains(os.Args, "__complete") && msg {
		if rest.UnreadMessages > 0 {
			msgs, err := rest.Messages(0, rest.UnreadMessages, true, true)
			if err != nil {
				log("Error getting messages: %v", err)
			} else {
				var disc int
				var data [][]any
				var ids []int
				for _, m := range msgs.Messages {
					if m.Type == "discovery" {
						ids = append(ids, m.ID)
						disc++
						continue
					}
					data = append(data, []any{m.Created.Time().Format(time.Kitchen), m.Title})
				}
				if disc > 0 {
					fmt.Printf("%d discovery messages\n", disc)
				}
				if len(data) > 0 {
					log("Messages:")
					printTable([]string{"Time", "Title"}, data)
				}
				if len(ids) > 0 {
					if err := rest.MarkRead(ids); err != nil {
						log("Error marking messages as read: %v", err)
					}
				}
			}
		}
		var ns []*models.Notification
		ns, err = models.PendingNotifications(false)
		if len(ns) > 0 {
			for _, n := range ns {
				if n.Device != "" {
					log("%s: %s -- %s", n.End, alias(n.Device), n.Text)
				} else {
					log("%s: %s", n.End, n.Text)
				}
			}
		}
	}
	if err != nil {
		die(err.Error())
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("raw", false, "emit the json returned")
	rootCmd.PersistentFlags().Bool("msg", true, "show unread message information")
}

var outputTable = map[string]func(data any) ([]string, [][]any){
	"default": func(data any) ([]string, [][]any) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]any{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		return []string{
				"Code", "Location", "Star", "Belt", "Status",
				"ETA", "Started", "Ends"},
			[][]any{{
				resp.DeviceCode, resp.Location, resp.Star,
				resp.Belt, resp.Status, resp.Eta.Duration(),
				resp.Started.Time(), resp.Completes.Time(),
			}}
	},
}
