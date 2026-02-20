package tools

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

type CalendarTool struct {
	url      string
	username string
	password string
}

func NewCalendarTool(url, username, password string) *CalendarTool {
	return &CalendarTool{url: url, username: username, password: password}
}

func (t *CalendarTool) Name() string {
	return "calendar"
}

func (t *CalendarTool) Description() string {
	return "Manage calendar events via CalDAV. Actions: list_calendars, list_events, get_event, create_event, update_event, delete_event."
}

func (t *CalendarTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "The action to perform: list_calendars, list_events, get_event, create_event, update_event, delete_event",
				"enum":        []string{"list_calendars", "list_events", "get_event", "create_event", "update_event", "delete_event"},
			},
			"calendars": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Calendar name(s) to target. Accepts one or more names. For list_events, queries all specified calendars. For create_event, uses the first. Defaults to the first calendar found if omitted.",
			},
			"start_date": map[string]any{
				"type":        "string",
				"description": "Start date for listing events, ISO 8601 format (e.g. 2025-01-15)",
			},
			"end_date": map[string]any{
				"type":        "string",
				"description": "End date for listing events, ISO 8601 format (e.g. 2025-01-31)",
			},
			"event_path": map[string]any{
				"type":        "string",
				"description": "Event resource path (for get_event, update_event, delete_event). Returned by list_events.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Event title (for create_event, update_event)",
			},
			"start": map[string]any{
				"type":        "string",
				"description": "Event start datetime, ISO 8601 (e.g. 2025-01-15T09:00:00Z). For all-day events use date only: 2025-01-15",
			},
			"end": map[string]any{
				"type":        "string",
				"description": "Event end datetime, ISO 8601 (e.g. 2025-01-15T10:00:00Z). For all-day events use date only: 2025-01-16",
			},
			"location": map[string]any{
				"type":        "string",
				"description": "Event location (for create_event, update_event)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Event description/notes (for create_event, update_event)",
			},
			"all_day": map[string]any{
				"type":        "boolean",
				"description": "If true, create an all-day event using date values for start/end",
			},
		},
		"required": []string{"action"},
	}
}

func (t *CalendarTool) DeclaredDomains() []string {
	u, err := url.Parse(t.url)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{u.Host}
}

func (t *CalendarTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return ErrorResult("action is required")
	}

	client, err := t.newClient()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create CalDAV client: %v", err))
	}

	switch action {
	case "list_calendars":
		return t.listCalendars(ctx, client)
	case "list_events":
		return t.listEvents(ctx, client, args)
	case "get_event":
		return t.getEvent(ctx, client, args)
	case "create_event":
		return t.createEvent(ctx, client, args)
	case "update_event":
		return t.updateEvent(ctx, client, args)
	case "delete_event":
		return t.deleteEvent(ctx, client, args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *CalendarTool) newClient() (*caldav.Client, error) {
	httpClient := webdav.HTTPClientWithBasicAuth(nil, t.username, t.password)
	return caldav.NewClient(httpClient, t.url)
}

func (t *CalendarTool) discoverCalendars(ctx context.Context, client *caldav.Client) ([]caldav.Calendar, error) {
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find principal: %w", err)
	}

	homeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("failed to find calendar home set: %w", err)
	}

	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil {
		return nil, fmt.Errorf("failed to find calendars: %w", err)
	}

	return calendars, nil
}

func (t *CalendarTool) resolveCalendars(ctx context.Context, client *caldav.Client, args map[string]any) ([]caldav.Calendar, error) {
	all, err := t.discoverCalendars(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no calendars found")
	}

	names := parseCalendarNames(args)
	if len(names) == 0 {
		return []caldav.Calendar{all[0]}, nil
	}

	byName := make(map[string]caldav.Calendar, len(all))
	for _, cal := range all {
		byName[strings.ToLower(cal.Name)] = cal
	}

	var result []caldav.Calendar
	var missing []string
	for _, name := range names {
		if cal, ok := byName[strings.ToLower(name)]; ok {
			result = append(result, cal)
		} else {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("calendar(s) not found: %s", strings.Join(missing, ", "))
	}

	return result, nil
}

func parseCalendarNames(args map[string]any) []string {
	raw, ok := args["calendars"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		names := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				names = append(names, s)
			}
		}
		return names
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func (t *CalendarTool) listCalendars(ctx context.Context, client *caldav.Client) *ToolResult {
	calendars, err := t.discoverCalendars(ctx, client)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to discover calendars: %v", err))
	}

	if len(calendars) == 0 {
		return SilentResult("No calendars found.")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d calendar(s):\n\n", len(calendars))
	for _, cal := range calendars {
		fmt.Fprintf(&b, "- %s\n", cal.Name)
		fmt.Fprintf(&b, "  Path: %s\n", cal.Path)
		if cal.Description != "" {
			fmt.Fprintf(&b, "  Description: %s\n", cal.Description)
		}
		if len(cal.SupportedComponentSet) > 0 {
			fmt.Fprintf(&b, "  Components: %s\n", strings.Join(cal.SupportedComponentSet, ", "))
		}
	}

	return SilentResult(b.String())
}

func (t *CalendarTool) listEvents(ctx context.Context, client *caldav.Client, args map[string]any) *ToolResult {
	calendars, err := t.resolveCalendars(ctx, client, args)
	if err != nil {
		return ErrorResult(err.Error())
	}

	startStr, _ := args["start_date"].(string)
	endStr, _ := args["end_date"].(string)

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)

	if startStr != "" {
		if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
			start = parsed
		} else if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = parsed
		}
	}
	if endStr != "" {
		if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
			end = parsed
		} else if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = parsed
		}
	}

	query := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{
			Name:    ical.CompCalendar,
			AllProps: true,
			Comps: []caldav.CalendarCompRequest{{
				Name:     ical.CompEvent,
				AllProps: true,
			}},
		},
		CompFilter: caldav.CompFilter{
			Name: ical.CompCalendar,
			Comps: []caldav.CompFilter{{
				Name:  ical.CompEvent,
				Start: start,
				End:   end,
			}},
		},
	}

	var b strings.Builder
	totalEvents := 0

	for _, cal := range calendars {
		objects, err := client.QueryCalendar(ctx, cal.Path, query)
		if err != nil {
			fmt.Fprintf(&b, "Error querying %q: %v\n\n", cal.Name, err)
			continue
		}

		if len(objects) == 0 {
			continue
		}

		fmt.Fprintf(&b, "## %s\n\n", cal.Name)
		for _, obj := range objects {
			if obj.Data == nil {
				continue
			}
			for _, event := range obj.Data.Events() {
				formatEventSummary(&b, obj.Path, &event)
				totalEvents++
			}
		}
	}

	if totalEvents == 0 {
		calNames := make([]string, len(calendars))
		for i, c := range calendars {
			calNames[i] = c.Name
		}
		return SilentResult(fmt.Sprintf("No events found in %s from %s to %s.", strings.Join(calNames, ", "), start.Format("2006-01-02"), end.Format("2006-01-02")))
	}

	header := fmt.Sprintf("Events from %s to %s:\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	return SilentResult(header + b.String())
}

func (t *CalendarTool) getEvent(ctx context.Context, client *caldav.Client, args map[string]any) *ToolResult {
	eventPath, ok := args["event_path"].(string)
	if !ok || eventPath == "" {
		return ErrorResult("event_path is required for get_event")
	}

	obj, err := client.GetCalendarObject(ctx, eventPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get event: %v", err))
	}

	if obj.Data == nil {
		return ErrorResult("event has no data")
	}

	events := obj.Data.Events()
	if len(events) == 0 {
		return ErrorResult("no event found at path")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Event details:\n\n")
	formatEventDetail(&b, obj.Path, &events[0])

	return SilentResult(b.String())
}

func (t *CalendarTool) createEvent(ctx context.Context, client *caldav.Client, args map[string]any) *ToolResult {
	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult("title is required for create_event")
	}

	startStr, _ := args["start"].(string)
	if startStr == "" {
		return ErrorResult("start is required for create_event")
	}

	endStr, _ := args["end"].(string)
	if endStr == "" {
		return ErrorResult("end is required for create_event")
	}

	allDay, _ := args["all_day"].(bool)
	location, _ := args["location"].(string)
	desc, _ := args["description"].(string)

	calendars, err := t.resolveCalendars(ctx, client, args)
	if err != nil {
		return ErrorResult(err.Error())
	}
	cal := &calendars[0]

	uid := newUID()

	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, uid)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	event.Props.SetText(ical.PropSummary, title)

	if allDay {
		startTime, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			return ErrorResult(fmt.Sprintf("invalid start date (expected YYYY-MM-DD for all-day): %v", err))
		}
		endTime, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			return ErrorResult(fmt.Sprintf("invalid end date (expected YYYY-MM-DD for all-day): %v", err))
		}
		event.Props.SetDate(ical.PropDateTimeStart, startTime)
		event.Props.SetDate(ical.PropDateTimeEnd, endTime)
	} else {
		startTime, err := parseDateTime(startStr)
		if err != nil {
			return ErrorResult(fmt.Sprintf("invalid start datetime: %v", err))
		}
		endTime, err := parseDateTime(endStr)
		if err != nil {
			return ErrorResult(fmt.Sprintf("invalid end datetime: %v", err))
		}
		event.Props.SetDateTime(ical.PropDateTimeStart, startTime)
		event.Props.SetDateTime(ical.PropDateTimeEnd, endTime)
	}

	if location != "" {
		event.Props.SetText(ical.PropLocation, location)
	}
	if desc != "" {
		event.Props.SetText(ical.PropDescription, desc)
	}

	calData := ical.NewCalendar()
	calData.Props.SetText(ical.PropVersion, "2.0")
	calData.Props.SetText(ical.PropProductID, "-//localagent//EN")
	calData.Children = append(calData.Children, event.Component)

	eventPath := cal.Path + uid + ".ics"
	_, err = client.PutCalendarObject(ctx, eventPath, calData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create event: %v", err))
	}

	return SilentResult(fmt.Sprintf("Event created: %s\nPath: %s\nCalendar: %s", title, eventPath, cal.Name))
}

func (t *CalendarTool) updateEvent(ctx context.Context, client *caldav.Client, args map[string]any) *ToolResult {
	eventPath, ok := args["event_path"].(string)
	if !ok || eventPath == "" {
		return ErrorResult("event_path is required for update_event")
	}

	obj, err := client.GetCalendarObject(ctx, eventPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get event for update: %v", err))
	}

	if obj.Data == nil {
		return ErrorResult("event has no data")
	}

	events := obj.Data.Events()
	if len(events) == 0 {
		return ErrorResult("no event found at path")
	}

	event := &events[0]

	if title, ok := args["title"].(string); ok && title != "" {
		event.Props.SetText(ical.PropSummary, title)
	}
	if location, ok := args["location"].(string); ok {
		if location == "" {
			event.Props.Del(ical.PropLocation)
		} else {
			event.Props.SetText(ical.PropLocation, location)
		}
	}
	if desc, ok := args["description"].(string); ok {
		if desc == "" {
			event.Props.Del(ical.PropDescription)
		} else {
			event.Props.SetText(ical.PropDescription, desc)
		}
	}

	allDay, _ := args["all_day"].(bool)

	if startStr, ok := args["start"].(string); ok && startStr != "" {
		if allDay {
			startTime, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				return ErrorResult(fmt.Sprintf("invalid start date: %v", err))
			}
			event.Props.SetDate(ical.PropDateTimeStart, startTime)
		} else {
			startTime, err := parseDateTime(startStr)
			if err != nil {
				return ErrorResult(fmt.Sprintf("invalid start datetime: %v", err))
			}
			event.Props.SetDateTime(ical.PropDateTimeStart, startTime)
		}
	}

	if endStr, ok := args["end"].(string); ok && endStr != "" {
		if allDay {
			endTime, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				return ErrorResult(fmt.Sprintf("invalid end date: %v", err))
			}
			event.Props.SetDate(ical.PropDateTimeEnd, endTime)
		} else {
			endTime, err := parseDateTime(endStr)
			if err != nil {
				return ErrorResult(fmt.Sprintf("invalid end datetime: %v", err))
			}
			event.Props.SetDateTime(ical.PropDateTimeEnd, endTime)
		}
	}

	event.Props.SetDateTime(ical.PropLastModified, time.Now().UTC())

	calData := ical.NewCalendar()
	calData.Props.SetText(ical.PropVersion, "2.0")
	calData.Props.SetText(ical.PropProductID, "-//localagent//EN")
	calData.Children = append(calData.Children, event.Component)

	_, err = client.PutCalendarObject(ctx, eventPath, calData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to update event: %v", err))
	}

	title, _ := event.Props.Text(ical.PropSummary)
	return SilentResult(fmt.Sprintf("Event updated: %s\nPath: %s", title, eventPath))
}

func (t *CalendarTool) deleteEvent(ctx context.Context, client *caldav.Client, args map[string]any) *ToolResult {
	eventPath, ok := args["event_path"].(string)
	if !ok || eventPath == "" {
		return ErrorResult("event_path is required for delete_event")
	}

	if err := client.RemoveAll(ctx, eventPath); err != nil {
		return ErrorResult(fmt.Sprintf("failed to delete event: %v", err))
	}

	return SilentResult(fmt.Sprintf("Event deleted: %s", eventPath))
}

func formatEventSummary(b *strings.Builder, path string, event *ical.Event) {
	summary, _ := event.Props.Text(ical.PropSummary)
	uid, _ := event.Props.Text(ical.PropUID)
	location, _ := event.Props.Text(ical.PropLocation)

	startTime, _ := event.DateTimeStart(nil)
	endTime, _ := event.DateTimeEnd(nil)

	isAllDay := false
	if prop := event.Props.Get(ical.PropDateTimeStart); prop != nil {
		if prop.ValueType() == ical.ValueDate {
			isAllDay = true
		}
	}

	fmt.Fprintf(b, "- %s\n", summary)
	fmt.Fprintf(b, "  Path: %s\n", path)
	if uid != "" {
		fmt.Fprintf(b, "  UID: %s\n", uid)
	}
	if isAllDay {
		fmt.Fprintf(b, "  Date: %s to %s (all day)\n", startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
	} else {
		fmt.Fprintf(b, "  Start: %s\n", startTime.Format(time.RFC3339))
		fmt.Fprintf(b, "  End: %s\n", endTime.Format(time.RFC3339))
	}
	if location != "" {
		fmt.Fprintf(b, "  Location: %s\n", location)
	}
	b.WriteString("\n")
}

func formatEventDetail(b *strings.Builder, path string, event *ical.Event) {
	summary, _ := event.Props.Text(ical.PropSummary)
	uid, _ := event.Props.Text(ical.PropUID)
	location, _ := event.Props.Text(ical.PropLocation)
	desc, _ := event.Props.Text(ical.PropDescription)
	status, _ := event.Props.Text(ical.PropStatus)

	startTime, _ := event.DateTimeStart(nil)
	endTime, _ := event.DateTimeEnd(nil)

	isAllDay := false
	if prop := event.Props.Get(ical.PropDateTimeStart); prop != nil {
		if prop.ValueType() == ical.ValueDate {
			isAllDay = true
		}
	}

	fmt.Fprintf(b, "Title: %s\n", summary)
	fmt.Fprintf(b, "Path: %s\n", path)
	if uid != "" {
		fmt.Fprintf(b, "UID: %s\n", uid)
	}
	if isAllDay {
		fmt.Fprintf(b, "Date: %s to %s (all day)\n", startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
	} else {
		fmt.Fprintf(b, "Start: %s\n", startTime.Format(time.RFC3339))
		fmt.Fprintf(b, "End: %s\n", endTime.Format(time.RFC3339))
	}
	if location != "" {
		fmt.Fprintf(b, "Location: %s\n", location)
	}
	if desc != "" {
		fmt.Fprintf(b, "Description: %s\n", desc)
	}
	if status != "" {
		fmt.Fprintf(b, "Status: %s\n", status)
	}
}

func newUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func parseDateTime(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse datetime %q (expected ISO 8601 format)", s)
}
