package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// Look up the requirements for an event
// If there are multiple options
//   check that we have all the required blueprints
//   require the user specify which one to take
// Check which of the requirements are still missing
// If there are missing components
//   check if we already have some available (with either no tag or event:id tag)
//   if not, check if they're already being printed with an 'event:id' tag
//   if not, queue them
//   ship them
// If there are missing resources
//   check if there are already any in transit
//   if not, ship them
// Once everything is in place
//   if there's a replicant there, resolve the event
//   if not, check if there's an ERM, and teleport to it
//   if not, send the nearest replicant there

///// V2
// Pick option -> empty event state
//   See which option is even possible
//   See the resource cost for each
//   Pick the cheaper
//
// Eval state -> fill in state
//   Resources: delivered/transit arrived/in transit/transit loaded/home
//   Devices: delivered/transit arrived/in transit/transit loaded/ready to load/printing/missing
//
// Actuate -> trigger actions to resolve
//   Handle resource freighters
//   Handle printing/platforms
//
// Complete
//   Reposition replicant
//   Complete event

type eventState struct {
	event       *models.Event
	tag         string
	txTag       string
	destination models.LocationID
	eta         map[string]time.Time
	transports  []*models.Device
	dryRun      bool

	required   map[string]int
	ready      map[string]int
	printing   map[string]int
	waiting    map[string][]*models.CodeAlias
	transitDev map[string][]*models.CodeAlias
	transitRes map[string]int
}

func newEventState(ev *models.Event, dryRun bool) *eventState {
	return &eventState{
		event:       ev,
		tag:         fmt.Sprintf("event:%s", strings.ToLower(ev.Designation)),
		txTag:       fmt.Sprintf("tx:%s", strings.ToLower(ev.Designation)),
		destination: ev.Location,
		dryRun:      dryRun,

		eta:        make(map[string]time.Time),
		required:   make(map[string]int),
		ready:      make(map[string]int),
		printing:   make(map[string]int),
		waiting:    make(map[string][]*models.CodeAlias),
		transitDev: make(map[string][]*models.CodeAlias),
		transitRes: make(map[string]int),
	}
}

func (es *eventState) selectOption(cID int) error {
	if cID > len(es.event.Criteria) {
		var opts []string
		for n, c := range es.event.Criteria {
			opts = append(opts, fmt.Sprintf("%d %s:\n%s", n, c.Name, c.Short()))
		}
		return fmt.Errorf("Invalid option %d selected, valid opts:\n%s", cID, strings.Join(opts, "\n\n"))
	}

	if cID == 0 {
		cID = 1
	}
	opt := es.event.Criteria[cID-1]
	log("Selected option: %s", opt.Short())
	maps.Copy(es.required, opt.Resources)
	for _, d := range opt.Devices {
		es.required[d.DeviceType] = d.Required
		es.printing[d.DeviceType] = 0
	}
	return nil
}

func (es *eventState) wait() time.Duration {
	var wait time.Duration
	for k, v := range es.eta {
		log("ETA[%s]: %s", k, v)
		if time.Until(v) > wait {
			wait = time.Until(v)
		}
	}

	return wait
}

func (es *eventState) later(kind string, t time.Time) {
	if t.After(es.eta[kind]) {
		es.eta[kind] = t
		log("Updated ETA[%s] to %s (%s)", kind, t, time.Until(t))
	}
}

func _dc(id *models.CodeAlias, cmd string, cfg map[string]any, dryRun bool) (*models.CommandResp, error) {
	if dryRun {
		log("[DRYRUN] %q -> %q (%v)", cmd, id, cfg)
		return new(models.CommandResp), nil
	}
	return rest.DeviceCommand[models.CommandResp](id, cmd, cfg)
}

func (es *eventState) unload(d *models.Device) error {
	res := make(map[string]int)
	for _, c := range d.Cargo {
		res[c.ResourceType] = c.Quantity
		es.ready[c.ResourceType] += c.Quantity
	}
	if len(res) > 0 {
		_, err := _dc(d.Code, "deposit_resources", map[string]any{"resources": res}, es.dryRun)
		if err != nil {
			return fmt.Errorf("Can't deposit cargo from %q: %v", d.Code, err)
		}
	}
	var targets []string
	for _, ad := range d.AttachedDevices {
		targets = append(targets, ad.Code.String())
		es.ready[d.Type]++
	}
	if len(targets) > 0 {
		_, err := _dc(d.Code, "detach", map[string]any{"targets": targets}, es.dryRun)
		if err != nil {
			return fmt.Errorf("Can't detach devices from %q: %v", d.Code, err)
		}
	}
	return nil
}

func (es *eventState) updateState() error {
	// Check what resources are already there
	log("Finding resources at %s", es.destination)
	inv, err := rest.Location(string(es.destination))
	if err != nil {
		return fmt.Errorf("Can't get inventory at %q: %v", es.destination, err)
	}
	for _, i := range inv.Inventory {
		es.ready[i.ResourceType] += i.Quantity
		log("... %s", i.String())
	}

	// Get all devices tagged for the event
	devs, err := rest.GetTagged(es.tag)
	if err != nil {
		return fmt.Errorf("Can't get %q devices: %v", es.tag, err)
	}
	log("Finding devices tagged %q:", es.tag)
	for _, d := range devs.Devices {
		log("... %s @ %s", d.Code.Alias(), d.Location)
		switch d.Location {
		case es.event.Location:
			es.ready[d.Type]++
		case home:
			es.waiting[d.Type] = append(es.waiting[d.Type], d.Code)
		default:
			es.transitDev[d.Type] = append(es.transitDev[d.Type], d.Code)
		}
	}

	// Get all event transports
	devs, err = rest.GetTagged(es.txTag)
	if err != nil {
		return fmt.Errorf("Can't get %q transports: %v", es.txTag, err)
	}
	for _, d := range devs.Devices {
		es.transports = append(es.transports, d)
		switch d.Location {
		case es.event.Location:
			log("%q is ready to unload")
			for _, c := range d.Cargo {
				es.ready[c.ResourceType] += c.Quantity
			}
		case home:
			log("%q is still loading")
			for _, c := range d.Cargo {
				es.waiting[c.ResourceType] = append(es.waiting[c.ResourceType], d.Code)
			}
		default:
			if d.Travel != nil {
				log("%q is in transit: ETA %s (%s)",
					d.Code, d.Travel.Arrives, time.Until(d.Travel.Arrives.Time()))
				es.later("transit", d.Travel.Arrives.Time())
			}
			for _, c := range d.Cargo {
				es.transitRes[c.ResourceType] += c.Quantity
			}
		}
	}
	log("Waiting: %v", es.waiting)
	log("Transit devs: %v", es.transitDev)
	log("Transit res: %v", es.transitRes)
	log("Ready: %v", es.ready)
	return nil
}

func (es *eventState) shipRes(res map[string]int) error {
	log("Shipping resources: %v", res)
	if len(res) == 0 {
		return nil
	}

	// Find free freighters at home
	cfs, err := rest.Devices(map[string]string{"location": home, "device_type": "cargo_freighter"})
	if err != nil {
		return fmt.Errorf("Error finding freighters: %v", err)
	}
	var errs []error
	for _, cf := range cfs {
		if len(cf.Tags) > 0 && cf.Tags[0] != es.txTag {
			continue
		}
		// if it's not already ours, it better be empty
		if len(cf.Tags) == 0 && len(cf.Cargo) > 0 {
			continue
		}
		avail := cf.CargoCapacity
		for _, c := range cf.Cargo {
			avail -= c.Quantity
		}
		if len(cf.Tags) == 0 {
			errs = append(errs, es.addTag(cf.Code, es.txTag))
		}
		manifest := make(map[string]int)
		for k, v := range res {
			if v <= avail {
				delete(res, k)
				manifest[k] = v
				avail -= v
			} else {
				res[k] -= avail
				manifest[k] = avail
				avail = 0
			}
			if avail == 0 {
				break
			}
		}
		_, err := _dc(cf.Code, "collect_resources", map[string]any{"resources": manifest}, es.dryRun)
		errs = append(errs, err)
		if err == nil {
			eta, err := common.Travel(cf.Code, string(es.destination), es.dryRun)
			errs = append(errs, err)
			es.later("resources", eta)
		}
		if len(res) == 0 {
			break
		}
	}
	if len(res) > 0 {
		errs = append(errs, fmt.Errorf("Not enough freighters available: %v remain", res))
	}

	return errors.Join(errs...)
}

func (es *eventState) shipDev(devs []*models.CodeAlias) error {
	if len(devs) == 0 {
		return nil
	}
	log("Shipping devices: %v", devs)

	// Find free platforms at home. Use smaller ones if we can.
	mfs, err := rest.Devices(map[string]string{"location": home, "device_type": "mobile_fleet"})
	if err != nil {
		return fmt.Errorf("Error finding fleets: %v", err)
	}
	if len(devs) <= 4 {
		plats, err := rest.Devices(map[string]string{"location": home, "device_type": "surge_platform"})
		if err != nil {
			return fmt.Errorf("Error finding platforms: %v", err)
		}
		mfs = append(plats, mfs...)
	}
	var errs []error
	for _, p := range mfs {
		if len(p.Tags) > 0 && p.Tags[0] != es.txTag {
			continue
		}
		avail := p.AttachCapacity - len(p.AttachedDevices)
		if avail <= 0 {
			continue
		}
		if len(p.Tags) == 0 {
			errs = append(errs, es.addTag(p.Code, es.txTag))
		}

		var ds []*models.CodeAlias
		if len(devs) <= avail {
			ds = devs[:]
			devs = devs[:0]
		} else {
			ds = devs[:avail]
			devs = devs[avail:]
		}
		_, err = _dc(p.Code, "attach", map[string]any{"targets": ds}, es.dryRun)
		errs = append(errs, err)

		if err == nil {
			eta, err := common.Travel(p.Code, string(es.destination), es.dryRun)
			errs = append(errs, err)
			es.later("devices", eta)
		}
		if len(devs) == 0 {
			break
		}
	}
	if len(devs) > 0 {
		errs = append(errs, fmt.Errorf("Not enough platforms available: %v remain", devs))
	}

	return errors.Join(errs...)
}

func (es *eventState) unTag(id *models.CodeAlias, tag string) error {
	if es.dryRun {
		log("[DRYRUN] Removing %q from %s", tag, id)
		return nil
	}
	_, err := rest.UpdateTags(id, rest.DelTag, []string{tag})
	return err
}

func (es *eventState) addTag(id *models.CodeAlias, tag string) error {
	if es.dryRun {
		log("[DRYRUN] Tagging %s with %q", id, tag)
		return nil
	}
	_, err := rest.UpdateTags(id, rest.AddTag, []string{tag})
	return err
}

// Complete
//
//	Reposition replicant
//	Complete event
func (es *eventState) complete() error {
	acc, err := rest.Account()
	if err != nil {
		return fmt.Errorf("Can't get account info: %v", err)
	}
	// Priority order:
	// 1. See if we just have someone already in the right place
	// 2. See if we can teleport there
	//    If there isn't add a TODO to install one
	// 3. Report distances from all replicants

	var rep *models.Replicant
	var data [][]any
	for _, r := range acc.ReplicantList {
		if r.CurrentLocation == es.destination {
			log("%s is already at %s.", r.Code, es.destination)
			if es.dryRun {
				log("[DRYRUN] Would complete event %s", es.event.Designation)
				return nil
			}
			return eventComplete(es.event.Designation)
		}

		// Save our teleport bunny for later. https://xkcd.com/221/
		if r.Code.Alias() == "r-4" {
			rep = r
		}

		// Get the vessel tags
		var tags []string
		hv, err := rest.DeviceInfo(r.HostedDeviceCode)
		if err == nil {
			tags = hv.Tags
		} else {
			log("Can't get vessel info: %v", err)
		}

		dist, err := common.Distance(r.Code.Alias(), es.destination.Star())
		if err != nil {
			log("Can't get distance to %s: %v", r.Code, err)
			dist = -1
		}
		data = append(data, []any{r.Code, r.CurrentLocation, dist, list(tags)})
	}
	if rep == nil {
		var repList []string
		for _, r := range acc.ReplicantList {
			repList = append(repList, fmt.Sprintf("%s @ %s", r.Code.Alias(), r.CurrentLocation))
		}
		return fmt.Errorf("Can't find our event volunteer:\n%s", strings.Join(repList, "\n"))
	}

	devs, err := getTeleportDests(string(es.destination))
	if err != nil {
		return fmt.Errorf("Can't get teleport destinations: %v", err)
	}
	if len(devs) > 0 {
		if es.dryRun {
			log("[DRYRUN] Would teleport to %s", devs[0])
			return nil
		}
		res, err := rest.ReplicantTeleport(rep.Code, devs[0].StowedDevices.Devices[0].Code)
		if err != nil {
			return fmt.Errorf("Can't teleport to %q: %v", devs[0].Code, err)
		}
		log("Waiting until %s (%s) for %s to wake up", res.Completes, time.Until(res.Completes.Time()), rep.Code)
		time.Sleep(time.Until(res.Completes.Time()))
		// Replicants sometime take a bit longer to wake up. Keep trying for up to 30s.
		deadline := time.Now().Add(30 * time.Second)
		for {
			err := eventComplete(es.event.Designation)
			if err == nil {
				return nil
			}
			if time.Now().After(deadline) {
				return err
			}
			time.Sleep(time.Second)
		}
	}

	log("Manual travel required to %s", es.destination)
	printTable([]string{"Replicant", "Location", "Distance LY", "Tags"}, data)
	return fmt.Errorf("Manual travel required")
}

// Actuate -> trigger actions to resolve
//
//	Handle resource freighters
//	Handle printing/platforms
func (es *eventState) actuate() error {
	var errs []error
	// Keep track of any resources in flight, unload ones that have arrived
	for _, t := range es.transports {
		if t.Location == es.destination {
			// Unload any deliveries that have arrived
			errs = append(errs, es.unload(t))
			// Untag and ship home
			errs = append(errs, es.unTag(t.Code, es.txTag))
			_, err := common.Travel(t.Code, home, es.dryRun)
			errs = append(errs, err)
		} else if t.Travel != nil {
			es.later("transit", t.Travel.Arrives.Time())
		}
	}

	// Figure out what resources and devices are missing
	toShip := make(map[string]int)
	toPrint := make(map[string]int)
	log("Checking for missing resources...")
	for k, v := range es.required {
		v -= es.ready[k]   // already delivered
		if isResource(k) { // already on the way
			v -= es.transitRes[k]
		} else {
			v -= len(es.transitDev[k])
			v -= len(es.waiting[k]) // ready for shipping
		}
		if v > 0 {
			log("... %d %s", v, k)
			if isResource(k) {
				toShip[k] = v
			} else {
				toPrint[k] = v
			}
		}
	}
	errs = append(errs, es.shipRes(toShip))

	var toLoad []*models.CodeAlias
	for _, v := range es.waiting {
		toLoad = append(toLoad, v...)
	}
	if len(toPrint) > 0 {
		// See if we have any spares at home
		log("Checking for available spares...")
		devs, err := rest.Devices(map[string]string{"location": home})
		if err != nil {
			errs = append(errs, err)
		}
		for _, d := range devs {
			if len(d.Tags) > 0 && d.Tags[0] != es.tag {
				continue
			}
			if need, ok := toPrint[d.Type]; ok {
				log("Reassigning %s to %s", d.Code, es.tag)
				toLoad = append(toLoad, d.Code)
				if need > 1 {
					toPrint[d.Type] = need - 1
				} else {
					delete(toPrint, d.Type)
				}
			}
		}
		// If we found spares, tag em
		for _, d := range toLoad {
			errs = append(errs, es.addTag(d, es.tag))
		}
	}

	// If we still need to print devices, check if they're already queued and
	// if not, print em
	for k, v := range toPrint {
		v -= common.CheckQueue(home, es.tag, k, v)
		if v <= 0 {
			log("%d %s already being printed", v, k)
			continue
		}
		cfg := map[string]any{
			"tags": []string{es.tag},
		}
		if bp := common.GetBP(k); bp != nil && slices.Contains(bp.Features, "modular") {
			cfg["flatpak"] = true
		}
		plan, err := common.Print(home, k, v, true, es.dryRun, cfg)
		errs = append(errs, err)
		es.later("printing", plan.ETA)
	}

	// If we're not printing anything else, ship em
	if len(toPrint) == 0 {
		errs = append(errs, es.shipDev(toLoad))
	}

	return errors.Join(errs...)
}

// Pick option -> empty event state
//
//	See which option is even possible
//	See the resource cost for each
//	Pick the cheapest
func pickCriteria(ev *models.Event, dryRun bool) (*eventState, error) {
	opts := ev.Criteria
	es := newEventState(ev, dryRun)

	if len(opts) == 1 {
		return es, es.selectOption(0)
	}

	type optCost struct {
		optID int
		res   int
		valid bool
	}
	cost := make([]optCost, len(opts))
	for n, opt := range opts {
		oc := optCost{
			optID: n,
			valid: true,
		}
		// add up the raw resource cost
		for _, v := range opt.Resources {
			oc.res += v
		}
		// make sure we have all the blueprints, add their cost
		for _, d := range opt.Devices {
			bp := common.GetBP(d.DeviceType)
			if bp == nil {
				oc.valid = false
				continue
			}
			res, err := bp.RawResources()
			if err != nil {
				oc.valid = false
				continue
			}
			for _, v := range res {
				oc.res += v
			}
		}
		cost[n] = oc
	}

	slices.SortFunc(cost, func(a, b optCost) int {
		return cmp.Compare(a.res, b.res)
	})

	for _, oc := range cost {
		if !oc.valid {
			continue
		}
		return es, es.selectOption(oc.optID)
	}

	return es, fmt.Errorf("No valid option found")
}

func autoEvent(cmd *cobra.Command, args []string) error {
	dryRun := getBool(cmd, "dry_run")
	res, err := rest.Events()
	if err != nil {
		return err
	}
	events := res.Events

	if id := getString(cmd, "id"); id != "" {
		var found bool
		for _, e := range events {
			if e.Designation == id {
				events = []*models.Event{e}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Can't find event %q", id)
		}
	} else if !getBool(cmd, "all") {
		return fmt.Errorf("Passing either --id or --all is required")
	}

	var errs []error
	var data [][]any
	for _, ev := range events {
		log("**** Processing event %s", ev.Designation)
		es, err := pickCriteria(ev, dryRun)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", ev.Designation, err))
			continue
		}
		if err := es.updateState(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", ev.Designation, err))
			continue
		}
		if err := es.actuate(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", ev.Designation, err))
			continue
		}
		wait := es.wait()
		if wait > 0 {
			wait = wait.Truncate(time.Second)
			data = append(data, []any{
				ev.Designation, ev.Location, ev.Title,
				fmt.Sprintf("Waiting: %s", wait.String()),
			})
			log("%s: Waiting until %s (%s)", ev.Designation, time.Now().Add(wait), wait)
			continue
		}
		if err := es.complete(); err != nil {
			data = append(data, []any{
				ev.Designation, ev.Location, ev.Title,
				fmt.Sprintf("Error: %v", err),
			})
			errs = append(errs, fmt.Errorf("%s: %v", ev.Designation, err))
			continue
		}
		data = append(data, []any{
			ev.Designation, ev.Location, ev.Title, "Complete",
		})
	}
	log("All done.")
	printTable([]string{"ID", "Location", "Title", "Status"}, data)

	return errors.Join(errs...)
}
