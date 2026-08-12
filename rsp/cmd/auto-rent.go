package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

func autoRent(cmd *cobra.Command, args []string) error {
	hubs, err := rest.Devices(map[string]string{
		"device_type": "system_hub",
	})
	if err != nil {
		return err
	}
	mds, err := rest.Devices(map[string]string{
		"device_type": "maintenance_drone",
	})
	if err != nil {
		return err
	}
	sbs, err := rest.Devices(map[string]string{
		"device_type": "service_bot",
	})
	if err != nil {
		return err
	}
	mds = append(mds, sbs...)
	mtds := make(map[string]*models.Device)
	for _, d := range mds {
		if d.AttachedToDeviceCode != nil {
			continue
		}
		if mtds[d.Location.Star()] != nil && mtds[d.Location.Star()].Status == "coordinating" {
			continue
		}
		mtds[d.Location.Star()] = d
	}
	atcStr := getString(cmd, "atc")
	home := getString(cmd, "home")
	dryRun := getBool(cmd, "dry_run")
	atc, err := getInfo(models.NewCodeAlias(atcStr))
	if err != nil {
		return err
	}
	ships := make(map[string]*models.Device)
	var wg sync.WaitGroup
	var mu sync.Mutex
	log("Fetching freighter information...")
	for _, cf := range atc.ControlledDevices {
		wg.Go(func() {
			fmt.Print(".")
			info, err := getInfo(cf.Code)
			if err != nil {
				log("Error getting info for %q: %v", cf.Code.Alias(), err)
				return
			}
			if info.Status != "idle" {
				return
			}

			mu.Lock()
			ships[info.Code.Alias()] = info
			mu.Unlock()
		})
	}
	wg.Wait()
	fmt.Println(". Done.")
	log("%d/%d ships available in rent fleet", len(ships), len(atc.ControlledDevices))

	type statusLine struct {
		cfs    []string
		status string
		cargo  map[string]int
		inv    map[string]int
		rent   map[string]int
	}
	var lines sync.Map
	initLine := func(loc string) {
		lines.LoadOrStore(loc, &statusLine{
			cfs:   []string{},
			cargo: make(map[string]int),
			inv:   make(map[string]int),
			rent:  make(map[string]int),
		})
	}

	deliver := func(loc string, inv map[string]int) (string, map[string]int, error) {
		// Find a ship
		var ship *models.Device

		mu.Lock()
		for _, cf := range ships {
			if len(cf.Cargo) != 0 {
				continue
			}
			if string(cf.Location) != home {
				continue
			}
			ship = cf
			break
		}
		if ship == nil {
			mu.Unlock()
			return "", nil, fmt.Errorf("%s: Can't find an available ship", loc)
		}
		// Remove the ship from our available list
		delete(ships, ship.Code.Alias())
		mu.Unlock()

		cf := "> " + ship.Code.Alias()

		// Load cargo
		if dryRun {
			log("Would load %v into %s", inv, ship.Code.Alias())
		} else {
			_, err := rest.DeviceCommand[models.CommandResp](ship.Code, "collect_resources",
				map[string]any{"resources": inv})
			if err != nil {
				return cf, nil, fmt.Errorf("Error loading %v into %s: %v", inv, ship.Code.Alias(), err)
			}
		}
		cargo := make(map[string]int)
		for k, v := range inv {
			cargo[k] += v
		}

		// Ship it
		if dryRun {
			log("Would ship %s to %s", ship.Code.Alias(), loc)
		} else if _, err := travel(ship.Code, loc); err != nil {
			return cf, nil, err
		}

		return cf, cargo, nil
	}

	res, err := rest.Location(home)
	log("Resources available at home:")
	for _, i := range res.Inventory {
		log("  %s: %d", i.ResourceType, i.Quantity)
	}

	// Find our ships that are not at home, deposit their cargo, and call back
	// Ships that are home should empty their holds
	var errs []error
	incoming := make(map[string]map[string]int)
	log("Finding remote freighters")
	for _, cf := range atc.ControlledDevices {
		wg.Go(func() {
			fmt.Print(".")
			info, err := rest.DeviceInfo(cf.Code)
			if err != nil {
				log("Error getting info for %q: %v", cf.Code.Alias(), err)
				return
			}
			if string(info.Location) == home {
				if len(info.Cargo) > 0 {
					if _, err := rest.DeviceCommand[models.CommandResp](info.Code, "deposit_resources", nil); err != nil {
						log("Error unloading cargo from %s: %v", info.Code, err)
					}
				}

				return
			}
			line := &statusLine{
				cfs:   []string{},
				cargo: make(map[string]int),
				inv:   make(map[string]int),
			}

			if info.Travel != nil {
				mu.Lock()
				dest := string(info.Travel.Destination)

				// Initialize the line if needed
				initLine(dest)

				if incoming[dest] == nil {
					incoming[dest] = make(map[string]int)
				}
				if dest != home {
					line.cfs = append(line.cfs, "> "+info.Code.Alias())
				} else {
					line.cfs = append(line.cfs, "< "+info.Code.Alias())
				}

				for _, c := range info.Cargo {
					incoming[dest][c.ResourceType] += int(c.Quantity)
					line.cargo[c.ResourceType] += int(c.Quantity)
				}
				mu.Unlock()
			}
			if info.Status != "idle" {
				return
			}
			if len(info.Cargo) > 0 {
				line.cfs = append(line.cfs, info.Code.Alias())
				for _, c := range info.Cargo {
					line.inv[c.ResourceType] += int(c.Quantity)
				}
				if dryRun {
					log("Would deposit contents of %s at %s", info.Code.Alias(), info.Location)
				} else if _, err := rest.DeviceCommand[models.CommandResp](info.Code, "deposit_resources", nil); err != nil {
					log("Deposited cargo from %s at %s", info.Code.Alias(), info.Location)
					errs = append(errs, err)
				}
			} else if dryRun {
				log("Would ship %s to %s", info.Code.Alias(), home)
			} else {
				_, err := travel(info.Code, home)
				errs = append(errs, err)
				log("Shiping %s to %s", info.Code.Alias(), home)
			}
			lines.Store(string(info.Location), line)
		})
	}
	wg.Wait()
	fmt.Println(". Done.")

	// Check hubs for missing resources, find a ship at home, load it, and send
	// it over
	var data [][]any
	slices.SortFunc(hubs, func(a, b *models.Device) int {
		return cmp.Or(
			cmp.Compare(a.Location.Star(), b.Location.Star()),
			cmp.Compare(a.Code.Num(), b.Code.Num()),
		)
	})
	log("Fetching information from system hubs")
	for _, sh := range hubs {
		wg.Go(func() {
			fmt.Print(".")
			sh, err := rest.DeviceInfo(sh.Code)
			if err != nil {
				errs = append(errs, fmt.Errorf("Can't refresh info for %q: %v", sh.Code, err))
			}
			line := &statusLine{
				rent:  make(map[string]int),
				inv:   make(map[string]int),
				cargo: make(map[string]int),
			}

			if st, ok := mtds[sh.Location.Star()]; !ok {
				errs = append(errs, fmt.Errorf("No maintenance drone found at %s for %s", sh.Location, sh.Code.Alias()))
			} else if st.Status != "coordinating" {
				dir := "patrol"
				if st.Type == "service_bot" {
					dir = "service"
				}
				err := setDirective(st.Code, dir, nil)
				if err != nil {
					errs = append(errs, fmt.Errorf("Failed to set %q to patrol: %v", st.Code.Alias(), err))
				}
			}
			if len(sh.UpkeepRequirements) == 0 {
				errs = append(errs, fmt.Errorf("Missing rent requirements for %s", sh.Code.Alias()))
			}
			inv, err := rest.Location(string(sh.Location))
			if err != nil {
				errs = append(errs, fmt.Errorf("Can't get resources at %s: %v", sh.Location, err))
				return
			}
			res := make(map[string]int)
			for _, i := range inv.Inventory {
				res[i.ResourceType] = int(i.Quantity)
				line.inv[i.ResourceType] += int(i.Quantity)
			}
			for _, r := range sh.UpkeepRequirements {
				res[r.ResourceType] -= r.QuantityPer20pct
				line.rent[r.ResourceType] += r.QuantityPer20pct
			}
			missing := make(map[string]int)
			for k, v := range res {
				if v >= 0 {
					continue
				}
				missing[k] -= v
			}
			if len(missing) > 0 {
				cf, cargo, err := deliver(string(sh.Location), missing)
				line.status = "rent due"
				line.cfs = append(line.cfs, cf)
				for k, v := range cargo {
					line.cargo[k] += v
				}
				errs = append(errs, err)
			} else {
				line.status = "up-to-date"
			}
			lines.Store(string(sh.Location), line)
		})
	}
	wg.Wait()
	fmt.Println(". Done.")
	c := func(in map[string]int) string {
		if in["carbon"] == 0 && in["structural"] == 0 {
			return ""
		}
		ca := fmt.Sprintf("%3d", in["carbon"])
		st := fmt.Sprintf("%3d", in["structural"])
		if in["carbon"] > 999 {
			ca = "inf"
		}
		if in["structural"] > 999 {
			st = "inf"
		}
		return fmt.Sprintf("c:%s,s:%s", ca, st)
	}
	for _, sh := range hubs {
		if sh.Status == "offline" || sh.Status == "compacted" {
			continue
		}
		line, ok := lines.Load(string(sh.Location))
		if !ok {
			continue
		}
		l := line.(*statusLine)
		data = append(data, []any{
			sh.Location, sh, l.status, p(sh.OperationalCapacity),
			c(l.inv), c(l.rent), c(l.cargo), list(l.cfs),
		})
	}
	if err := errors.Join(errs...); err != nil {
		log("Couldn't pay all the rent:\n%v", err)
	}
	printTable([]string{"Location", "Hub", "Status", "Ops", "Inventory", "Rent", "Cargo", "CFs"}, data)

	return nil
}
