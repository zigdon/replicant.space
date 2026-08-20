package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
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
	defer db.DB.Close()

	// if the first arg looks like a device alias, assume "device -d"
	args := os.Args
	if db.Dealias(args[1]) != args[1] {
		args = slices.Insert(args, 1, "device", "-d")
	} else if args[1] == "device" && db.Dealias(args[2]) != args[2] {
		args = slices.Insert(args, 2, "-d")
	}
	rootCmd.SetArgs(args[1:])

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
				skipped := make(map[string]int)
				var data [][]any
				var ids []int
				for _, m := range msgs.Messages {
					if slices.Contains([]string{"discovery", "notification"}, m.Type) {
						ids = append(ids, m.ID)
						skipped[m.Type]++
						continue
					}
					if m.Type == "achievement" && strings.HasPrefix(m.Title, "Event completed") {
						ids = append(ids, m.ID)
						skipped["event completed"]++
						continue
					}
					data = append(data, []any{m.Created.Time().Format(time.Kitchen), m.Title})
				}
				if len(skipped) > 0 {
					for k, v := range skipped {
						fmt.Printf("%d %s messages\n", v, k)
					}
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
			var silent int
			for _, n := range ns {
				if n.Device != "" {
					log("%s: %s -- %s", n.End, alias(n.Device), n.Text)
				} else {
					silent++
				}
			}
			if silent > 0 {
				log("%d silent notifications suppressed", silent)
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
