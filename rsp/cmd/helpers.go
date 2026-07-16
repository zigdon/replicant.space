package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type flagDesc struct {
	name      string
	short     rune
	value     any
	desc      string
	required  bool
	slice     bool
	jsonKey   string
	mapFlag   bool
	intFlag   bool
	boolFlag  bool
	rangeFlag bool
	valueFn   func(*models.CodeAlias, any) any
}

func mkCommand[T any](parent *cobra.Command, name, short, command string, flags []flagDesc, output string) *cobra.Command {
	if output == "" {
		output = "default"
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("device")
			ca := models.NewCodeAlias(id)
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
					if f.valueFn != nil {
						val = f.valueFn(ca, val).([]string)
					}
				} else if f.intFlag {
					val, _ = cmd.Flags().GetInt(f.name)
					if f.valueFn != nil {
						val = f.valueFn(ca, val).(int)
					}
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
					if f.valueFn != nil {
						val = f.valueFn(ca, val).(map[string]string)
					}
				} else if f.boolFlag {
					val, _ = cmd.Flags().GetBool(f.name)
					if f.valueFn != nil {
						val = f.valueFn(ca, val).(bool)
					}
				} else {
					val, _ = cmd.Flags().GetString(f.name)
					if f.valueFn != nil {
						val = f.valueFn(ca, val).(string)
					}
				}
				if f.name == "repeat" {
					if val.(int) > 0 {
						reps = val.(int)
						log("Repeating %d times\n", reps)
					}
					continue
				}
				if f.rangeFlag {
					val = explode(val)
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
				resp, err := rest.DeviceCommand[T](ca, command, data)
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
				cmd.Flags().IntP(f.name, string(f.short), val, f.desc)
			} else {
				cmd.Flags().Int(f.name, val, f.desc)
			}
		} else if f.boolFlag {
			val, _ := f.value.(bool)
			if f.short != 0 {
				cmd.Flags().BoolP(f.name, string(f.short), val, f.desc)
			} else {
				cmd.Flags().Bool(f.name, val, f.desc)
			}
		} else {
			val, _ := f.value.(string)
			if f.short != 0 {
				cmd.Flags().StringP(f.name, string(f.short), val, f.desc)
			} else {
				cmd.Flags().String(f.name, val, f.desc)
			}
		}
		if f.required && f.valueFn == nil {
			cmd.MarkFlagRequired(f.name)
		}
	}
	return cmd
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

func aliases(in []*models.CodeAlias) []string {
	res := make([]string, len(in))
	for i, ca := range in {
		res[i] = ca.Alias()
	}
	return res
}

func unalias(in string) string {
	if db == nil {
		return in
	}
	return db.Dealias(in)
}

func explode[T any](v T) []string {
	var res []string
	var in []string
	if s, ok := any(v).(string); ok {
		in = []string{s}
	} else if s, ok := any(v).([]string); ok {
		in = s
	} else {
		panic(fmt.Errorf("Invalid type %v", v))
	}
	for len(in) > 0 {
		s := in[0]
		if len(in) > 1 {
			in = in[1:]
		} else {
			in = []string{}
		}
		if spl := strings.Split(s, ","); len(spl) > 1 {
			in = append(in, spl...)
			continue
		}
		if parts := strings.Split(s, "-"); len(parts) == 3 {
			start, err := strconv.Atoi(parts[1])
			if err != nil {
				panic(err)
			}
			end, err := strconv.Atoi(parts[2])
			if err != nil {
				panic(err)
			}
			if start >= end {
				panic(fmt.Errorf("invalid range: %d >= %d", start, end))
			}
			for i := start; i <= end; i++ {
				in = append(in, fmt.Sprintf("%s-%d", parts[0], i))
			}
			continue
		}
		res = append(res, s)
	}

	return res
}

var bps map[string]*models.Blueprint

func getBP(bp string) *models.Blueprint {
	if bps == nil {
		bps = make(map[string]*models.Blueprint)
	}
	if b, ok := bps[bp]; ok {
		return b
	}
	b := &models.Blueprint{DeviceType: bp}
	if err := b.Get(); err != nil {
		panic(fmt.Sprintf("Can load blueprint for %s: %v", bp, err))
	}
	bps[bp] = b
	return b
}
