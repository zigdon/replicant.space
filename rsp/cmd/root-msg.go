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
	ids, _ := cmd.Flags().GetIntSlice("ids")
	if len(ids) > 0 {
		partial = true
	}
	width, _ := cmd.Flags().GetInt("width")
	cursor, _ := cmd.Flags().GetInt("cursor")
	number, _ := cmd.Flags().GetInt("number")
	latest, _ := cmd.Flags().GetBool("latest")
	readToo, _ := cmd.Flags().GetBool("read")
	data, err := rest.Messages(cursor, number, latest, !readToo)
	if err != nil {
		return fmt.Errorf("Error getting status: %v", err)
	}
	if raw, _ := cmd.Flags().GetBool("raw"); raw {
		prettyPrint(data)
	} else {
		var msgs [][]string
		for _, m := range data.Messages {
			if !partial {
				ids = append(ids, m.ID)
			}
			msgs = append(msgs, []string{
				d(m.ID),
				m.Type,
				wrap(m.Title, 20),
				wrap(m.Body, width),
				b(m.Read),
				t(m.Created.Time()),
			})
		}
		printTable([]string{"ID", "Type", "Title", "Body", "Read", "Created"}, msgs)

		if mark, _ := cmd.Flags().GetBool("mark"); partial || mark {
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
		// Find a relay we can use
		acc, err := rest.Account()
		if err != nil {
			return err
		}

		var relayID string
		for _, r := range acc.Replicants {
			devices, err := rest.ReplicantDevices(r.Code, "")
			if err != nil {
				continue
			}
			for _, d := range devices {
				if d.Type != "ftl_relay" || d.Status != "relaying" || !d.InControlRange {
					continue
				}
				relayID = d.Code.String()
				break
			}
			if relayID != "" {
				break
			}
		}

		if relayID == "" {
			return fmt.Errorf("Failed to find an FTL relay in range")
		}

		cursor, _ := cmd.Flags().GetInt("cursor")
		number, _ := cmd.Flags().GetInt("number")
		latest, _ := cmd.Flags().GetBool("latest")
		npcs, _ := cmd.Flags().GetBool("npcs")
		width, _ := cmd.Flags().GetInt("width")
		ids, _ := cmd.Flags().GetBool("replicant_ids")
		locs, _ := cmd.Flags().GetBool("replicant_location")
		channels, _ := cmd.Flags().GetStringSlice("channels")
		data, err := rest.Bobnet(relayID, cursor, number, latest, npcs)
		if err != nil {
			return fmt.Errorf("Error getting bobnet messages: %v", err)
		}
		headers := []string{"Channel", "Name", "Time", "Message"}
		var lines [][]string
		slices.Reverse(data.Messages)
		for _, d := range data.Messages {
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
			lines = append(lines, []string{
				d.Channel, who, t(d.Time.Time()), wrap(d.Message, width),
			})
		}
		printTable(headers, lines)
		return nil
	},
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
	bobCmd.Flags().IntP("width", "w", 50, "Wrap message body to this width")
	bobCmd.Flags().BoolP("npcs", "p", true, "Show messages from NPCs")
	bobCmd.Flags().Bool("replicant_ids", false, "Show replicant IDs")
	bobCmd.Flags().Bool("replicant_location", false, "Show replicant locations")
	bobCmd.Flags().StringSliceP("channels", "c", []string{}, "Only show messages to these channels")

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
			NewCell(true, dt(time.Until(msg.Created.Time()))).
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
		msgWin.Clear().SetTitle(msg.Title)
		printTablef(msgWin, []string{
			"Created", "Type", "ID", "Title",
		}, [][]string{{
			t(msg.Created.Time()), msg.Type, d(msg.ID), msg.Title,
		}})
		printTablef(msgWin, []string{"Body"}, [][]string{{
			wrap(msg.Body, 80)}})
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
	listWin.SetSelectedFunc(markReadCell).
		SetCell(0, 0, NewCell(false, "When").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 1, NewCell(false, "Type").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetCell(0, 2, NewCell(false, "Title").SetAlign(tview.AlignCenter).SetStyle(titleStyle)).
		SetFixed(1, 0)

	logWin := newLogWindow()
	msgWin.SetBorder(true)
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
