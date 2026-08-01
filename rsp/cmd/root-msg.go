package cmd

import (
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
	Short: "Read messages from bobnet",
	RunE: func(cmd *cobra.Command, args []string) error {
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
	},
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
	bobCmd.Flags().BoolP("latest", "l", true, "Show latest messages")
	bobCmd.Flags().IntP("number", "n", 20, "Number of messages to show")
	bobCmd.Flags().IntP("cursor", "C", 0, "Position to start from")
	bobCmd.PersistentFlags().IntP("width", "w", 50, "Wrap message body to this width")
	bobCmd.PersistentFlags().BoolP("npcs", "p", true, "Show messages from NPCs")
	bobCmd.PersistentFlags().Bool("replicant_ids", false, "Show replicant IDs")
	bobCmd.PersistentFlags().Bool("replicant_location", false, "Show replicant locations")
	bobCmd.PersistentFlags().StringSliceP("channels", "c", []string{}, "Only show messages to these channels")
	bobCmd.PersistentFlags().StringP("relay", "r", "fr-1", "Relay to use for sending the message")

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
