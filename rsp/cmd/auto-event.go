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

type replicantTask struct {
	vessel *models.CodeAlias
	rep    *models.CodeAlias
	dest   models.LocationID
	ev     *models.Event
	dist   float32
}

var tasks []replicantTask

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
		log("... %s @ %s", d.Code, d.Location)
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
			log("%q is ready to unload", d)
			for _, c := range d.Cargo {
				es.ready[c.ResourceType] += c.Quantity
			}
		case home:
			log("%q is still loading", d)
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
		cf, _ = rest.DeviceInfo(cf.Code)
		log("Checking %s", cf.Code)
		if len(cf.Tags) > 0 && cf.Tags[0] != es.txTag {
			log("... %s: has tags: %v", cf.Code, cf.Tags)
			continue
		}
		// if it's not already ours, it better be empty
		if len(cf.Tags) == 0 && len(cf.Cargo) > 0 {
			log("... %s: not empty: %v", cf.Code, cf.Cargo)
			continue
		}
		avail := cf.CargoCapacity
		for _, c := range cf.Cargo {
			avail -= c.Quantity
		}
		log("... %s: %d available", cf.Code, avail)
		if len(cf.Tags) == 0 {
			log("... %s: tagging %s", cf.Code, es.txTag)
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
		if len(manifest) == 0 {
			return fmt.Errorf("Attempting to load an empty manifest onto %s", cf.Code)
		}
		log("Loading %v onto %s", manifest, cf.Code)
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
	var unloaded []*models.CodeAlias
	var loaded []*models.CodeAlias
	for _, d := range devs {
		info, err := rest.DeviceInfo(d)
		if err != nil {
			return err
		}
		if info.AttachedToDeviceCode != nil {
			loaded = append(loaded, info.AttachedToDeviceCode)
			continue
		}
		unloaded = append(unloaded, d)
	}
	devs = unloaded
	var errs []error
	if len(devs) > 0 {
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
			log("Attaching %d devices to %s", len(ds), p.Code)
			_, err = _dc(p.Code, "attach", map[string]any{"targets": ds}, es.dryRun)
			errs = append(errs, err)
			if err == nil {
				loaded = append(loaded, p.Code)
			}
			if len(devs) == 0 {
				break
			}
		}
		if len(devs) > 0 {
			errs = append(errs,
				fmt.Errorf("Not enough platforms available: %d (%v) remain", len(devs), devs))
		}
	}

	sent := make(map[string]bool)
	for _, p := range loaded {
		if sent[p.Alias()] {
			continue
		}
		eta, err := common.Travel(p, string(es.destination), es.dryRun)
		errs = append(errs, err)
		es.later("devices", eta)
		sent[p.Alias()] = true
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
	// 3. See if there's a matrix container nearby that can be moved over
	// 4. Report distances from all replicants

	var rep *models.Replicant
	var data [][]any
	var nearest float32 = -1
	var name string
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

		// If there's an 'auto' tag on the vessel, don't consider it an option.
		var tags []string
		hv, err := rest.DeviceInfo(r.HostedDeviceCode)
		if err == nil {
			tags = hv.Tags
			if slices.Contains(tags, "auto") {
				continue
			}
		} else {
			log("Can't get vessel info: %v", err)
		}

		dist, err := common.Distance(r.Code.Alias(), es.destination.Star())
		if err != nil {
			log("Can't get distance to %s: %v", r.Code, err)
			dist = -1
		} else if r.CurrentLocation != "" && (nearest < 0 || dist < nearest) {
			nearest = dist
			name = r.Code.Alias()
			tasks = append(tasks, replicantTask{
				vessel: hv.Code,
				rep:    r.Code,
				dest:   es.destination,
				ev:     es.event,
				dist:   dist,
			})
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

	log("Checking for nearby containers that can be moved over")
	devs, err = rest.Devices(map[string]string{"device_type": "matrix_container"})
	if err != nil {
		return fmt.Errorf("Can't get matrix containers: %v", err)
	}
	dists := make(map[string]float32)
	mcLocs := make(map[string][]*models.Device)
	hasMC := make(map[string]bool)
	var nearestMC float32 = -1
	var spf *models.CodeAlias

	for _, d := range devs {
		d, err := rest.DeviceInfo(d.Code)
		if err != nil {
			log("Error getting info for %q: %v", d.Code, err)
			continue
		}

		// If it's already in motion, ignore it
		if d.Location == "" {
			continue
		}
		star := d.Location.Star()

		// If it doesn't actually host a matrix, skip it
		if d.StowedDevices == nil || len(d.StowedDevices.Devices) == 0 {
			continue
		}
		// If it's not attached to another device (e.g. spf), skip it, but note that system has one
		if d.AttachedToDeviceCode == nil {
			hasMC[star] = true
			continue
		}

		dist, err := common.Distance(d.Code.Alias(), es.destination.Star())
		if err != nil {
			log("Can't get distance from %q: %v", d.Location, err)
			continue
		}
		dists[star] = dist
		mcLocs[star] = append(mcLocs[star], d)
	}

	// Look through the list of mobile MCs.
	// - If there's already one deployed in that system, we can move the
	//   platform.
	// - If not, deploy one, then if there are any left on the platform,
	//   - If the platform has _another_ MC, move it.
	//   - If not, send the platform home.
	for loc, mcs := range mcLocs {
		// No local MC, deploy it
		if !hasMC[loc] {
			mc := mcs[0]
			mcs = mcs[1:]
			log("Deploying %s at %s", mc, loc)
			_, err := _dc(mc.AttachedToDeviceCode, "detach", map[string]any{"target": mc.Code}, es.dryRun)
			if err != nil {
				log("Can't detach %q from %q: %v", mc.Code, mc.AttachedToDeviceCode, err)
				continue
			}
			hasMC[loc] = true
			if len(mcs) > 0 {
				mcLocs[loc] = mcs
			} else {
				log("Sending the empty %q back home", mc.AttachedToDeviceCode)
				common.Travel(mc.AttachedToDeviceCode, home, es.dryRun)
				continue
			}
		}
		// If we got here, there was at least one more MC available to move over
		if nearestMC < 0 || dists[loc] < nearestMC {
			nearestMC = dists[loc]
			spf = mcs[0].AttachedToDeviceCode
			log("%s can be relocated", spf)
		}
	}

	if spf != nil {
		log("Sending %s with a matrix container to %s", spf, es.event.Location)
		if eta, err := common.Travel(spf, string(es.event.Location), es.dryRun); err != nil {
			log("Error shipping %q: %v", spf, err)
		} else {
			return fmt.Errorf(
				"Shipped matrix to %s, ETA %s (%s)", es.event.Location,
				eta.Truncate(time.Second).Format(time.Kitchen),
				time.Until(eta).Truncate(time.Second))
		}
	}

	log("Manual travel required to %s", es.destination)
	slices.SortFunc(data, func(a, b []any) int {
		da := a[2].(float32)
		db := b[2].(float32)
		return cmp.Compare(da, db)
	})
	printTable([]string{"Replicant", "Location", "Distance LY", "Tags"}, data)
	return fmt.Errorf("Manual travel required: %s is nearest (%.2f LY from %s)",
		name, nearest, es.destination)
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
		destDevs, err := rest.Devices(map[string]string{"location": string(es.destination)})
		if err != nil {
			errs = append(errs, err)
		}
		devs = append(devs, destDevs...)
		for _, d := range devs {
			if _, ok := toPrint[d.Type]; !ok {
				continue
			}
			d, err = rest.DeviceInfo(d.Code)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if len(d.Tags) > 0 && d.Tags[0] == es.tag {
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

	// Make sure all modular devices are compacted before shipping
	for _, d := range toLoad {
		info, err := rest.DeviceInfo(d)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if slices.Contains(info.Features, "modular") && info.Status != "compacted" {
			res, err := _dc(d, "compact", nil, es.dryRun)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			log("Compacting %s: ETA %s (%s)", d, res.Completes.Time(), time.Until(res.Completes.Time()))
			es.later("compacting", res.Completes.Time())
		}
	}

	// If we still need to print devices, check if they're already queued and
	// if not, print em
	for k, v := range toPrint {
		printing, printEta := common.CheckQueue(home, es.tag, k, v)
		if v-printing <= 0 {
			log("%d %s already being printed", v, k)
			es.later("printing", printEta)
			continue
		}
		cfg := map[string]any{
			"tags": []string{es.tag},
		}
		if bp := common.GetBP(k); bp != nil && slices.Contains(bp.Features, "modular") {
			cfg["flatpack"] = true
		}
		plan, err := common.Print(home, k, v, true, es.dryRun, cfg)
		errs = append(errs, err)
		es.later("printing", plan.ETA)
	}

	// If we're not printing anything else, and we're not waiting for flackpacks, ship em
	if len(toPrint) == 0 && time.Now().After(es.eta["compacting"]) {
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

func eventCleanup(currentEvents []*models.Event, dryRun bool) error {
	evTags := make(map[string]bool)
	for _, e := range currentEvents {
		evTags[fmt.Sprintf("event:%s", strings.ToLower(e.Designation))] = true
		evTags[fmt.Sprintf("tx:%s", strings.ToLower(e.Designation))] = true
	}

	// get all known event tags (both event:* and tx:*)
	// if a remote freighter/platform, ship it home (without unloading first)
	// if it's at home, unload and untag
	// if it's a remote device, just untag it (and it'll probably get reused for a future event)
	rows, err := db.DB.Query(`
		SELECT DISTINCT(JSONB_ARRAY_ELEMENTS_TEXT( data->'tags')) AS tags
		FROM json_devices`)
	if err != nil {
		return err
	}
	var cleanup []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		if !strings.HasPrefix(t, "event:") && !strings.HasPrefix(t, "tx:") {
			continue
		}
		if evTags[t] {
			continue
		}
		cleanup = append(cleanup, t)
	}
	log("Found %d tags to clean up", len(cleanup))

	unload := func(d *models.Device) error {
		log("... unloading %s", d.Code)
		if d.HasCapability("attach") && len(d.AttachedDevices) > 0 {
			if _, err := _dc(d.Code, "detach", nil, dryRun); err != nil {
				return err
			}
		}
		if d.HasCapability("transport") && len(d.Cargo) > 0 {
			if _, err := _dc(d.Code, "deposit_resources", nil, dryRun); err != nil {
				return err
			}
		}

		return nil
	}

	untag := func(d *models.Device, t string) error {
		if dryRun {
			log("[DRYRUN] Untagging %q from %s", t, d.Code)
			return nil
		}
		log("... untagging %q %s", t, d.Code)
		_, err := rest.UpdateTags(d.Code, rest.DelTag, []string{t})
		return err
	}

	shipped := make(map[string]bool)
	goHome := func(d *models.CodeAlias) error {
		if shipped[d.Alias()] {
			return nil
		}
		log("... shipping %s home", d)
		_, err := common.Travel(d, home, dryRun)
		shipped[d.Alias()] = true
		return err
	}

	var errs []error
	done := make(map[string]bool)
	for _, t := range cleanup {
		log("Cleaning up %q:", t)
		devs, err := rest.GetTagged(t)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, d := range devs.Devices {
			if d.Location == "" {
				log("... %s in transit", d.Code)
				continue
			}
			if string(d.Location) == home {
				errs = append(errs, untag(d, t))
			}

			if done[d.Code.Alias()] {
				continue
			}

			if string(d.Location) == home {
				errs = append(errs, unload(d))
			} else if d.HasCapability("surge") {
				errs = append(errs, goHome(d.Code))
			} else if d.AttachedToDeviceCode != nil {
				errs = append(errs, goHome(d.AttachedToDeviceCode))
			} else {
				errs = append(errs, untag(d, t))
			}
			done[d.Code.Alias()] = true
		}
	}

	return errors.Join(errs...)
}

func autoEvent(cmd *cobra.Command, args []string) error {
	dryRun := getBool(cmd, "dry_run")
	res, err := rest.Events()
	if err != nil {
		return err
	}
	events := res.Events

	if err := eventCleanup(events, dryRun); err != nil {
		log("**** Error cleaning up obsolete tags: %v", err)
	}

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
				fmt.Sprintf("Waiting: %s (%s)",
					wait.String(), time.Now().Add(wait).Truncate(time.Second).Format(time.Kitchen)),
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
	if err := errors.Join(errs...); err != nil {
		log("Execution errors:\n%v\n\n", err)
	}
	printTable([]string{"ID", "Location", "Title", "Status"}, data)

	if getBool(cmd, "ship_replicants") {
		log("Auto-shipping enabled...")
		sent := make(map[string]bool)
		slices.SortFunc(tasks, func(a, b replicantTask) int {
			return cmp.Compare(a.dist, b.dist)
		})
		for _, t := range tasks {
			if t.dist > 100 {
				break
			}
			if sent[t.rep.Alias()] {
				continue
			}
			sent[t.rep.Alias()] = true
			log("... Sending %s in %s on a trip to %s (%.2f LY away) to complete %s",
				t.rep, t.vessel, t.dest, t.dist, t.ev.Designation)
			if _, err := common.Travel(t.vessel, string(t.dest), dryRun); err != nil {
				log("Shipping failed: %v", err)
			}
		}
	}

	return nil
}
