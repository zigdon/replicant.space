package cmd

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var msgCmd = &cobra.Command{
	Use:     "msg",
	Aliases: []string{"msgs"},
	Short:   "Interactive message browser",
	RunE:    msgTable,
}

func msgList(cmd *cobra.Command, args []string) error {
	var partial bool
	ids := getIntSlice(cmd, "ids")
	if len(ids) > 0 {
		partial = true
	}
	width := getInt(cmd, "width")
	cursor := getInt(cmd, "cursor")
	number := getInt(cmd, "number")
	latest := getBool(cmd, "latest")
	readToo := getBool(cmd, "read")
	data, err := rest.Messages(cursor, number, latest, !readToo)
	if err != nil {
		return fmt.Errorf("Error getting status: %v", err)
	}
	if raw := getBool(cmd, "raw"); raw {
		prettyPrint(data)
	} else {
		var msgs [][]any
		for _, m := range data.Messages {
			if !partial {
				ids = append(ids, m.ID)
			}
			msgs = append(msgs, []any{
				m.ID,
				m.Type,
				wrap(m.Title, 20),
				wrap(m.Body, width),
				m.Read,
				m.Created.Time(),
			})
		}
		printTable([]string{"ID", "Type", "Title", "Body", "Read", "Created"}, msgs)

		if mark := getBool(cmd, "mark"); partial || mark {
			log("Marking messages read: %v", ids)
			if err := rest.MarkRead(ids); err != nil {
				log("Error marking messages read: %v", err)
			}
		}
	}
	return nil
}

var bobCmd = &cobra.Command{
	Use:   "bob",
	Short: "Interactive message viewer for bobnet",
	RunE:  bobTable,
}

var bobTableCmd = &cobra.Command{
	Use:     "table",
	Aliases: []string{"browse", "view"},
	Short:   "Interactive bobnet message viewer",
	RunE:    bobTable,
}

var bobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List bobnet messages",
	RunE:  bobList,
}

func bobList(cmd *cobra.Command, args []string) error {
	relayID := models.NewCodeAlias(getString(cmd, "relay"))
	cursor := getInt(cmd, "cursor")
	number := getInt(cmd, "number")
	latest := getBool(cmd, "latest")
	npcs := getBool(cmd, "npcs")
	data, err := rest.Bobnet(relayID, cursor, number, latest, npcs)
	if err != nil {
		return fmt.Errorf("Error getting bobnet messages: %v", err)
	}
	printBobMsgs(cmd, data.Messages)
	return nil
}

func printBobMsgs(cmd *cobra.Command, msgs []*models.Bob) {
	width := getInt(cmd, "width")
	ids := getBool(cmd, "replicant_ids")
	locs := getBool(cmd, "replicant_location")
	channels := getStringSlice(cmd, "channels")
	headers := []string{"Channel", "Name", "Time", "Message"}
	var lines [][]any
	slices.Reverse(msgs)
	for _, d := range msgs {
		if len(channels) > 0 && !slices.Contains(channels, d.Channel) {
			continue
		}
		var who string
		if ids || locs {
			d.ReplicantCode = "#" + d.ReplicantCode
			d.CurrentStar = "@" + d.CurrentStar
			who = fmt.Sprintf("%s (%s%s)", d.ReplicantName, d.ReplicantCode, d.CurrentStar)
		} else {
			who = d.ReplicantName
		}
		lines = append(lines, []any{
			d.Channel, who, d.Time.Time(), wrap(d.Message, width),
		})
	}
	printTable(headers, lines)
}

var bobSendCmd = &cobra.Command{
	Use:       "send",
	Short:     "Send a message to bobnet",
	ValidArgs: []string{"#general", "#trade"},
	RunE:      bobSend,
}

var msgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List messages",
	RunE:  msgList,
}

func init() {
	rootCmd.AddCommand(msgCmd)

	msgCmd.AddCommand(bobCmd)
	bobCmd.PersistentFlags().BoolP("latest", "l", true, "Show latest messages")
	bobCmd.PersistentFlags().IntP("number", "n", 20, "Number of messages to show")
	bobCmd.PersistentFlags().IntP("cursor", "C", 0, "Position to start from")
	bobCmd.PersistentFlags().IntP("width", "w", 50, "Wrap message body to this width")
	bobCmd.PersistentFlags().BoolP("npcs", "p", true, "Show messages from NPCs")
	bobCmd.PersistentFlags().Bool("replicant_ids", false, "Show replicant IDs")
	bobCmd.PersistentFlags().Bool("replicant_location", false, "Show replicant locations")
	bobCmd.PersistentFlags().StringSliceP("channels", "c", []string{}, "Only show messages to these channels")
	bobCmd.PersistentFlags().StringP("relay", "r", "fr-1", "Relay to use for sending the message")

	bobCmd.AddCommand(bobListCmd)
	bobCmd.AddCommand(bobTableCmd)
	bobCmd.AddCommand(bobSendCmd)
	bobSendCmd.Flags().BoolP("listen", "l", false, "If set, remain connected to bobnet to see replies")

	msgCmd.AddCommand(msgListCmd)
	msgListCmd.Flags().BoolP("mark", "m", false, "Mark messages as read")
	msgListCmd.Flags().BoolP("latest", "l", true, "Show latest messages")
	msgListCmd.Flags().BoolP("read", "r", false, "Show also read messages")
	msgListCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	msgListCmd.Flags().IntP("cursor", "C", 0, "Position to start from")
	msgListCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	msgListCmd.Flags().IntSlice("ids", []int{}, "Mark these messages as read")
}

func loadUnreadMsgs() ([]*models.Message, error) {
	var res []*models.Message
	for {
		var ids []int
		msgs, err := rest.Messages(0, 50, false, true)
		if err != nil {
			return nil, err
		}
		if len(msgs.Messages) == 0 {
			break
		}
		for _, m := range msgs.Messages {
			ids = append(ids, m.ID)
			res = append(res, m)
		}
		if err := rest.MarkRead(ids); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func msgTable(cmd *cobra.Command, args []string) error {
	listWin := tview.NewTable().
		SetSelectable(true, false)
	msgWin := tview.NewTextView()
	onlyUnread := true
	filterType := ""
	var msgTypes []string

	app := tview.NewApplication()
	setMsgLine := func(n int, msg *models.Message) {
		style := tcell.StyleDefault
		if !msg.Read {
			style = style.Bold(true).Foreground(tcell.ColorGreen)
		}
		listWin.SetCell(n, 0,
			NewCell(true, common.Dt(time.Until(msg.Created.Time()))).
				SetStyle(style).
				SetReference(msg))
		listWin.SetCell(n, 1, NewCell(true, msg.Type).SetStyle(style))
		listWin.SetCell(n, 2, NewCell(true, msg.Title).SetStyle(style))
	}
	getMessages := func() {
		_, err := loadUnreadMsgs()
		if err != nil {
			log("Error loading new messages: %v", err)
		}
		msgs, err := db.ListIDs(cache.MsgTable)
		if err != nil {
			log("Error getting IDs: %v", err)
		}

		ids := cache.Ints(msgs)
		slices.Sort(ids)
		for listWin.GetRowCount() > 1 {
			listWin.RemoveRow(1)
		}

		line := 1
		filterCnt := 0
		for _, id := range ids {
			msg := &models.Message{ID: int(id)}
			if err := msg.Get(); err != nil {
				log("Failed to load message %d: %v", id, err)
				continue
			}
			if !slices.Contains(msgTypes, msg.Type) {
				msgTypes = append(msgTypes, msg.Type)
			}
			if onlyUnread && msg.Read {
				filterCnt++
				continue
			}
			if filterType != "" && msg.Type != filterType {
				filterCnt++
				continue
			}
			line++
			setMsgLine(line, msg)
		}
		slices.Sort(msgTypes)
		var descBits []string
		if onlyUnread {
			descBits = append(descBits, "unread")
		}
		if filterType != "" {
			descBits = append(descBits, filterType)
		}
		desc := strings.Join(descBits, ", ")
		if len(desc) > 0 {
			desc += " "
		}
		log("Showing %d %smessages (%d filtered)", line-1, desc, filterCnt)
	}
	displayCell := func(row, col int) {
		ref := listWin.GetCell(row, 0).GetReference()
		if ref == nil {
			return
		}
		msg := ref.(*models.Message)
		msgWin.Clear().SetTitle(fmt.Sprintf("  %s  ", msg.Title))
		fmt.Fprintf(msgWin, "%s (%s ago) %-20s\n\n",
			msg.Created.Time().Truncate(time.Second).Format(time.Stamp),
			time.Since(msg.Created.Time()).Truncate(time.Second), msg.Type,
		)
		fmt.Fprintf(msgWin, "%s", msg.Body)
	}
	markReadCell := func(row, col int) {
		ref := listWin.GetCell(row, 0).GetReference()
		if ref == nil {
			return
		}
		msg := ref.(*models.Message)
		msg.Read = !msg.Read
		if err := msg.Cache(); err != nil {
			log("Error saving read status: %v", err)
			return
		}
		setMsgLine(row, msg)
	}
	markReadAll := func() {
		if filterType == "" {
			log("Marking all messages as read")
		} else {
			log("Marking all %s messages as read", filterType)
		}
		for row := 1; row < listWin.GetRowCount(); row++ {
			ref := listWin.GetCell(row, 0).GetReference()
			if ref == nil {
				continue
			}
			msg := ref.(*models.Message)
			if filterType != "" && msg.Type != filterType {
				continue
			}
			msg.Read = true
			if err := msg.Cache(); err != nil {
				log("Error saving read status: %v", err)
				return
			}
			setMsgLine(row, msg)
		}
	}
	listWin.SetSelectionChangedFunc(displayCell).
		SetBorder(true)
	titleStyle := tcell.StyleDefault.Underline(true)
	listWin.SetBorderPadding(1, 1, 1, 1)
	listWin.SetSelectedFunc(markReadCell).
		SetCell(0, 0, NewCell(false, "When").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 1, NewCell(false, "Type").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 2, NewCell(false, "Title").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetFixed(1, 0)

	logWin := newLogWindow()
	msgWin.SetBorder(true).SetBorderPadding(2, 2, 2, 2)
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(listWin, 0, 1, true).
			AddItem(msgWin, 0, 2, false), 0, 1, true).
		AddItem(logWin, 10, 0, false)
	getMessages()
	listWin.Select(listWin.GetRowCount()-1, 0)
	inputCapture := func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Rune() == 'a':
			markReadAll()
		case ev.Rune() == 'r':
			getMessages()
		case ev.Rune() == 'u':
			onlyUnread = !onlyUnread
			getMessages()
		case ev.Rune() == 't':
			if len(msgTypes) == 0 || filterType == msgTypes[len(msgTypes)-1] {
				filterType = ""
			} else {
				filterType = msgTypes[slices.Index(msgTypes, filterType)+1]
			}
			getMessages()
		case ev.Rune() == 'q':
			app.Stop()
		}
		// Only allow keystroke handling if we actually have messages to view.
		if listWin.GetRowCount() > 1 {
			return ev
		}
		return nil
	}
	app.SetInputCapture(inputCapture)

	return app.SetRoot(layout, true).Run()
}

func bobSend(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Usage: msg bob send <channel> <msg>")
	}
	channel := args[0]
	msg := strings.Join(args[1:], " ")
	if !strings.HasPrefix(channel, "#") {
		channel = "#" + channel
	}
	relay := models.NewCodeAlias(getString(cmd, "relay"))
	res, err := rest.BobSend(relay, channel, msg)
	if err != nil {
		return err
	}
	if res.Status != "sent" {
		log("Message not sent: %q", res.Status)
	}

	printMsg := func(msg *models.Bob) {
		log("%20s %10s %10s %s", msg.Channel, msg.ReplicantName,
			msg.Time.Time().Format(time.Kitchen), msg.Message)
	}
	printMsg(res)

	var last = res.Id
	if getBool(cmd, "listen") {
		for {
			time.Sleep(5 * time.Second)
			msgs, err := rest.Bobnet(relay, last, 10, true, true)
			if err != nil {
				return err
			}
			if len(msgs.Messages) > 0 {
				slices.Reverse(msgs.Messages)
				for _, m := range msgs.Messages {
					if m.Id <= last {
						continue
					}
					printMsg(m)
				}
			}
			last = msgs.NextCursor
		}
	}
	return nil
}

func backfillBobnet(relayID *models.CodeAlias) error {
	if db == nil {
		return fmt.Errorf("Not connected to cache")
	}

	var maxID int
	err := db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM bobnet_messages").Scan(&maxID)
	if err != nil {
		return err
	}

	cursor := maxID
	for {
		data, err := rest.Bobnet(relayID, cursor, 50, false, true)
		if err != nil {
			return err
		}
		if len(data.Messages) == 0 {
			break
		}
		newCount := 0
		for _, m := range data.Messages {
			if m.Id > maxID {
				newCount++
			}
			if err := m.Cache(); err != nil {
				log("Error caching message %d: %v", m.Id, err)
			}
		}
		if data.NextCursor <= cursor || data.NextCursor == 0 || newCount == 0 {
			break
		}
		cursor = data.NextCursor
	}
	return nil
}

func getBobnetChannels() []string {
	if db == nil {
		return nil
	}
	rows, err := db.Query(`
	    SELECT DISTINCT channel
		FROM bobnet_messages
		WHERE channel != ''
		ORDER BY channel ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var channels []string
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err == nil {
			channels = append(channels, ch)
		}
	}
	return channels
}

func getBobnetReplicants() []string {
	if db == nil {
		return nil
	}
	rows, err := db.Query(`
	    SELECT DISTINCT sender_name
		FROM bobnet_messages
		WHERE sender_name != ''
		ORDER BY sender_name ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var replicants []string
	for rows.Next() {
		var rep string
		if err := rows.Scan(&rep); err == nil {
			replicants = append(replicants, rep)
		}
	}
	return replicants
}

func queryBobnetMessages(channel, replicant string) ([]*models.Bob, error) {
	if db == nil {
		return nil, fmt.Errorf("Not connected to cache")
	}
	var where []string
	var args []any
	n := 1
	if channel != "" {
		where = append(where, fmt.Sprintf("channel = $%d", n))
		args = append(args, channel)
		n++
	}
	if replicant != "" {
		where = append(where, fmt.Sprintf("sender_name = $%d", n))
		args = append(args, replicant)
		n++
	}

	q := "SELECT id, channel, sender_name, sender_code, star, message, time, status FROM bobnet_messages"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY time ASC"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.Bob
	for rows.Next() {
		m := new(models.Bob)
		var t time.Time
		var sc, st, status sql.NullString
		if err := rows.Scan(&m.Id, &m.Channel, &m.ReplicantName, &sc, &st, &m.Message, &t, &status); err != nil {
			return nil, err
		}
		if sc.Valid {
			m.ReplicantCode = sc.String
		}
		if st.Valid {
			m.CurrentStar = st.String
		}
		if status.Valid {
			m.Status = status.String
		}
		m.Time = models.NewJsonTime(t)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func bobTable(cmd *cobra.Command, args []string) error {
	relayID := models.NewCodeAlias(getString(cmd, "relay"))
	showIDs := getBool(cmd, "replicant_ids")
	showLocs := getBool(cmd, "replicant_location")

	activeChannel := "#general"
	initialChannels := getStringSlice(cmd, "channels")
	if len(initialChannels) > 0 {
		activeChannel = initialChannels[0]
	}
	activeReplicant := ""

	listWin := tview.NewTable().SetSelectable(true, false)
	msgWin := tview.NewTextView()
	helpBar := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	sendInput := tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray).
		SetFieldTextColor(tcell.ColorWhite)
	logWin := newLogWindow()

	app := tview.NewApplication()
	pages := tview.NewPages()
	isSending := false

	updateHelpBar := func() {
		chDesc := activeChannel
		if chDesc == "" {
			chDesc = "All"
		}
		repDesc := activeReplicant
		if repDesc == "" {
			repDesc = "All"
		}
		helpBar.SetText(fmt.Sprintf(
			" [yellow]q[-] Quit  [yellow]r[-] Refresh  [yellow]c[-] Channel ([green]%s[-])  [yellow]u[-] Replicant ([green]%s[-])  [yellow]i[-] IDs  [yellow]s/Enter[-] Send",
			chDesc, repDesc,
		))
	}

	setMsgLine := func(n int, m *models.Bob) {
		style := tcell.StyleDefault
		var who string
		if showIDs || showLocs {
			code := m.ReplicantCode
			if code != "" {
				code = "#" + code
			}
			star := m.CurrentStar
			if star != "" {
				star = "@" + star
			}
			who = fmt.Sprintf("%s (%s%s)", m.ReplicantName, code, star)
		} else {
			who = m.ReplicantName
		}

		when := ""
		if m.Time != nil {
			when = common.Dt(time.Until(m.Time.Time()))
		}

		preview := strings.ReplaceAll(m.Message, "\n", " ")

		listWin.SetCell(n, 0,
			NewCell(true, when).
				SetStyle(style).
				SetReference(m))
		listWin.SetCell(n, 1, NewCell(true, m.Channel).SetStyle(style))
		listWin.SetCell(n, 2, NewCell(true, who).SetStyle(style))
		listWin.SetCell(n, 3, NewCell(true, preview).SetStyle(style))
	}

	displayCell := func(row, col int) {
		ref := listWin.GetCell(row, 0).GetReference()
		if ref == nil {
			return
		}
		m := ref.(*models.Bob)
		msgWin.Clear().SetTitle(fmt.Sprintf("  %s - %s  ", m.Channel, m.ReplicantName))

		var timeStr, agoStr string
		if m.Time != nil {
			timeStr = m.Time.Time().Truncate(time.Second).Format(time.Stamp)
			agoStr = fmt.Sprintf("(%s ago)", time.Since(m.Time.Time()).Truncate(time.Second))
		}

		fmt.Fprintf(msgWin, "%s %s %s\n", timeStr, agoStr, m.Channel)
		if m.ReplicantCode != "" || m.CurrentStar != "" {
			fmt.Fprintf(msgWin, "From: %s (#%s @ %s)\n\n", m.ReplicantName, m.ReplicantCode, m.CurrentStar)
		} else {
			fmt.Fprintf(msgWin, "From: %s\n\n", m.ReplicantName)
		}
		if m.Status != "" {
			fmt.Fprintf(msgWin, "Status: %s\n\n", m.Status)
		}
		fmt.Fprintf(msgWin, "%s", m.Message)
	}

	getMessages := func() {
		if err := backfillBobnet(relayID); err != nil {
			log("Backfill error: %v", err)
		}
		msgs, err := queryBobnetMessages(activeChannel, activeReplicant)
		if err != nil {
			log("Query error: %v", err)
			return
		}

		for listWin.GetRowCount() > 1 {
			listWin.RemoveRow(1)
		}

		line := 1
		for _, m := range msgs {
			line++
			setMsgLine(line, m)
		}

		updateHelpBar()

		chDesc := activeChannel
		if chDesc == "" {
			chDesc = "All"
		}
		repDesc := activeReplicant
		if repDesc == "" {
			repDesc = "All"
		}
		log("Showing %d messages (Channel: %s, Replicant: %s)", len(msgs), chDesc, repDesc)

		if listWin.GetRowCount() > 1 {
			listWin.Select(listWin.GetRowCount()-1, 0)
		} else {
			msgWin.Clear().SetTitle(" Message Details ")
			fmt.Fprintf(msgWin, "No messages matching channel %q and replicant %q", chDesc, repDesc)
		}
	}

	titleStyle := tcell.StyleDefault.Underline(true)
	listWin.SetSelectionChangedFunc(displayCell).
		SetBorder(true).
		SetTitle(" Bobnet Messages ")
	listWin.SetBorderPadding(1, 1, 1, 1)
	listWin.
		SetCell(0, 0, NewCell(false, "When").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 1, NewCell(false, "Channel").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 2, NewCell(false, "From").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 3, NewCell(false, "Message").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetFixed(1, 0)

	msgWin.SetBorder(true).SetBorderPadding(2, 2, 2, 2).SetTitle(" Message Details ")

	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(listWin, 0, 1, true).
			AddItem(msgWin, 0, 1, false), 0, 1, true).
		AddItem(helpBar, 1, 0, false).
		AddItem(logWin, 10, 0, false)

	pages.AddPage("main", mainFlex, true, true)

	modalView := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	showChannelModal := func() {
		channels := getBobnetChannels()
		list := tview.NewList().ShowSecondaryText(false)

		list.AddItem("(All Channels)", "", 0, func() {
			activeChannel = ""
			pages.RemovePage("channel_modal")
			app.SetFocus(listWin)
			getMessages()
		})
		for _, ch := range channels {
			cName := ch
			list.AddItem(cName, "", 0, func() {
				activeChannel = cName
				pages.RemovePage("channel_modal")
				app.SetFocus(listWin)
				getMessages()
			})
		}
		list.SetDoneFunc(func() {
			pages.RemovePage("channel_modal")
			app.SetFocus(listWin)
		})
		list.SetBorder(true).SetTitle(" Select Channel ").SetTitleAlign(tview.AlignCenter)
		height := len(channels) + 5
		if height > 20 {
			height = 20
		}
		pages.AddPage("channel_modal", modalView(list, 40, height), true, true)
		app.SetFocus(list)
	}

	showReplicantModal := func() {
		replicants := getBobnetReplicants()
		list := tview.NewList().ShowSecondaryText(false)

		list.AddItem("(All Replicants)", "", 0, func() {
			activeReplicant = ""
			pages.RemovePage("replicant_modal")
			app.SetFocus(listWin)
			getMessages()
		})
		for _, rep := range replicants {
			rName := rep
			list.AddItem(rName, "", 0, func() {
				activeReplicant = rName
				pages.RemovePage("replicant_modal")
				app.SetFocus(listWin)
				getMessages()
			})
		}
		list.SetDoneFunc(func() {
			pages.RemovePage("replicant_modal")
			app.SetFocus(listWin)
		})
		list.SetBorder(true).SetTitle(" Select Replicant ").SetTitleAlign(tview.AlignCenter)
		height := len(replicants) + 5
		if height > 20 {
			height = 20
		}
		pages.AddPage("replicant_modal", modalView(list, 40, height), true, true)
		app.SetFocus(list)
	}

	startSending := func() {
		targetChannel := activeChannel
		if targetChannel == "" {
			row, _ := listWin.GetSelection()
			if row > 0 && row < listWin.GetRowCount() {
				ref := listWin.GetCell(row, 0).GetReference()
				if ref != nil {
					targetChannel = ref.(*models.Bob).Channel
				}
			}
			if targetChannel == "" {
				targetChannel = "#general"
			}
		}

		isSending = true
		sendInput.SetLabel(fmt.Sprintf(" [yellow]Send to %s:[-] ", targetChannel))
		sendInput.SetText("")
		sendInput.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				text := strings.TrimSpace(sendInput.GetText())
				if text != "" {
					res, err := rest.BobSend(relayID, targetChannel, text)
					if err != nil {
						log("Failed to send message: %v", err)
					} else {
						log("Sent message (%s): %s", targetChannel, text)
						if err := res.Cache(); err != nil {
							log("Error caching sent message: %v", err)
						}
						getMessages()
					}
				}
			}
			isSending = false
			mainFlex.RemoveItem(sendInput)
			mainFlex.AddItem(helpBar, 1, 0, false)
			app.SetFocus(listWin)
		})

		mainFlex.RemoveItem(helpBar)
		mainFlex.AddItem(sendInput, 1, 0, true)
		app.SetFocus(sendInput)
	}

	listWin.SetSelectedFunc(func(row, col int) {
		startSending()
	})

	inputCapture := func(ev *tcell.EventKey) *tcell.EventKey {
		if isSending {
			if ev.Key() == tcell.KeyEscape {
				isSending = false
				mainFlex.RemoveItem(sendInput)
				mainFlex.AddItem(helpBar, 1, 0, false)
				app.SetFocus(listWin)
				return nil
			}
			return ev
		}

		if pages.HasPage("channel_modal") || pages.HasPage("replicant_modal") {
			if ev.Key() == tcell.KeyEscape {
				pages.RemovePage("channel_modal")
				pages.RemovePage("replicant_modal")
				app.SetFocus(listWin)
				return nil
			}
			return ev
		}

		switch {
		case ev.Rune() == 'q' || ev.Key() == tcell.KeyEscape:
			app.Stop()
			return nil
		case ev.Rune() == 'r':
			getMessages()
			return nil
		case ev.Rune() == 'c':
			showChannelModal()
			return nil
		case ev.Rune() == 'u' || ev.Rune() == 'm':
			showReplicantModal()
			return nil
		case ev.Rune() == 'i':
			showIDs = !showIDs
			getMessages()
			return nil
		case ev.Rune() == 's':
			startSending()
			return nil
		}

		if listWin.GetRowCount() > 1 {
			return ev
		}
		return nil
	}
	app.SetInputCapture(inputCapture)

	getMessages()

	return app.SetRoot(pages, true).Run()
}

