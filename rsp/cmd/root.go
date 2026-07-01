package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
	"github.com/zigdon/rsp/tui"
)

type flagDesc struct {
	name     string
	short    rune
	value    any
	desc     string
	required bool
	slice    bool
	jsonKey  string
	mapFlag  bool
	intFlag  bool
}

var db *cache.Cache

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rsp",
	Short: "Simple cli for interacting with replicant.space",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Connect to the database
	var err error
	db, err = cache.Connect(false)
	if err != nil {
		log("Failed to connect to db: %v", err)
	} else {
		models.ConnectDB(db)
		rest.ConnectDB(db)
	}

	// Create any missing aliases
	_, err = rest.AllDevices()

	err = rootCmd.Execute()
	if err != nil {
		die(err.Error())
	}
	if rest.UnreadMessages > 0 {
		log("Unread messages: %d", rest.UnreadMessages)
	}
	ns, err := models.PendingNotifications(false)
	if len(ns) > 0 {
		for _, n := range ns {
			if n.Device != "" {
				log("%s: %s -- %s", n.End.Round(time.Second).String(),
					alias(n.Device), n.Text)
			} else {
				log("%s: %s", n.End.Round(time.Second).String(), n.Text)
			}
		}
	}
	if err != nil {
		die(err.Error())
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("raw", false, "emit the json returned")
	rootCmd.AddCommand(tui.TUI)
}

var outputTable = map[string]func(data any) ([]string, [][]string){
	"default": func(data any) ([]string, [][]string) {
		resp, ok := data.(*models.CommandResp)
		if !ok {
			return []string{"Type error"}, [][]string{{fmt.Sprintf("Can't convert %v to CommandResp", data)}}
		}
		return []string{
				"Code", "Location", "Star", "Belt", "Status",
				"ETA", "Started", "Ends"},
			[][]string{{
				resp.DeviceCode.Alias(), resp.Location, resp.Star,
				resp.Belt, resp.Status, resp.Eta.String(), resp.Started.String(), resp.Completes.String(),
			}}
	},
}

var mkCommand = func(parent *cobra.Command, name, short, command string, flags []flagDesc, output string) *cobra.Command {
	if output == "" {
		output = "default"
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("device")
			data := make(map[string]any)
			var argsFlag flagDesc
			var reps = 1
			for _, f := range flags {
				if f.name == "" {
					argsFlag = f
				}
				if f.jsonKey == "" {
					f.jsonKey = f.name
				}

				var val any
				if f.slice {
					val, _ = cmd.Flags().GetStringSlice(f.name)
				} else if f.intFlag {
					val, _ = cmd.Flags().GetInt(f.name)
				} else if f.mapFlag {
					ms, _ := cmd.Flags().GetStringSlice(f.name)
					if len(ms) == 0 {
						continue
					}
					dataMap := make(map[string]string)
					for _, mv := range ms {
						bits := strings.Split(mv, ":")
						dataMap[bits[0]] = bits[1]
					}
					val = dataMap
				} else {
					val, _ = cmd.Flags().GetString(f.name)
				}
				if f.name == "repeat" {
					if val.(int) > 0 {
						reps = val.(int)
						log("Repeating %d times\n", reps)
					}
					continue
				}
				if f.required {
					data[f.jsonKey] = val
				} else if val != "" {
					data[f.jsonKey] = val
				}
			}
			if argsFlag.jsonKey != "" {
				if len(args) == 0 || args[0] == "" {
					return fmt.Errorf("Argument is required for %q", name)
				}
				data[argsFlag.jsonKey] = args[0]
			}
			var repData [][]string
			var repHeaders []string
			for range reps {
				resp, err := rest.DeviceCommand(models.NewCodeAlias(id), command, data)
				if err != nil {
					return fmt.Errorf("Error sending %q to %q: %v", command, id, err)
				}
				if raw, _ := cmd.Flags().GetBool("raw"); raw {
					prettyPrint(resp)
					continue
				}
				outFn, ok := outputTable[output]
				if !ok {
					return fmt.Errorf("Output format not found: %q", output)
				}
				headers, data := outFn(resp)
				repHeaders = headers
				repData = append(repData, data...)
			}
			if len(repData) > 0 {
				printTable(repHeaders, repData)
			}
			return nil
		},
	}
	parent.AddCommand(cmd)
	for _, f := range flags {
		if f.name == "" {
			continue
		}
		if f.slice || f.mapFlag {
			val, _ := f.value.(string)
			if f.short != 0 {
				cmd.Flags().StringSliceP(f.name, string(f.short), []string{val}, f.desc)
			} else {
				cmd.Flags().StringSlice(f.name, []string{val}, f.desc)
			}
		} else if f.intFlag {
			val, _ := f.value.(int)
			if f.short != 0 {
				cmd.Flags().IntSliceP(f.name, string(f.short), []int{val}, f.desc)
			} else {
				cmd.Flags().IntSlice(f.name, []int{val}, f.desc)
			}
		} else {
			val, _ := f.value.(string)
			if f.short != 0 {
				cmd.Flags().StringP(f.name, string(f.short), val, f.desc)
			} else {
				cmd.Flags().String(f.name, val, f.desc)
			}
		}
		if f.required {
			cmd.MarkFlagRequired(f.name)
		}
	}
	return cmd
}

var chainCmd = func(a, b *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   a.Use,
		Short: a.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.RunE(cmd, args); err != nil {
				return err
			}
			log("Chaining command: %s", b.Short)
			return b.RunE(cmd, args)
		},
	}
}

func aliasType(in string) (string, string) {
	if db == nil {
		return "", ""
	}
	return db.GetAliasAndType(in)
}

func alias(in string) string {
	if db == nil {
		return in
	}
	// Check if there's already an alias
	out := db.HasAlias(in)
	if out != "" {
		return out
	}

	// If it doesn't look like a code, don't try to look it up
	if strings.ToUpper(in) != in {
		return in
	}

	// No alias, get the device type before making one
	deviceType, err := rest.GetType(in)
	if err != nil || deviceType == "" {
		return in
	}
	out, err = db.Alias(in, deviceType)
	if err != nil {
		log("Error creating alias for %q(%s): %v", in, deviceType, err)
	}
	return out
}

func unalias(in string) string {
	if db == nil {
		return in
	}
	return db.Dealias(in)
}
