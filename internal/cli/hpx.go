package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/toto/withingy/internal/activity"
	"github.com/toto/withingy/internal/api"
	"github.com/toto/withingy/internal/measures"
	"github.com/toto/withingy/internal/sleep"
	"github.com/toto/withingy/internal/workouts"
)

const (
	hpxFlag      = "hpx"
	hpxFlagSince = "since"
	hpxFlagUntil = "until"
	hpxFlagLast  = "last"
	hpxFlagLimit = "limit"

	hpxSource = "withings"
)

var hpxNow = time.Now

func init() {
	rootCmd.Flags().Bool(hpxFlag, false, "Emit Hypercontext NDJSON to stdout")
	rootCmd.Flags().String(hpxFlagSince, "", "Lower time bound for --hpx (RFC3339 or YYYY-MM-DD)")
	rootCmd.Flags().String(hpxFlagUntil, "", "Upper time bound for --hpx (RFC3339 or YYYY-MM-DD)")
	rootCmd.Flags().String(hpxFlagLast, "", "Relative export window for --hpx (examples: 72h, 10d, 1mo)")
	rootCmd.Flags().Int(hpxFlagLimit, 0, "Limit exported records per source family for --hpx (0 exports all)")
}

type hpxOptions struct {
	rangeOpts *api.ListOptions
	limit     int
}

type hpxSignpostRecord struct {
	Table    string         `json:"table"`
	Kind     string         `json:"kind"`
	TS       string         `json:"ts"`
	Edge     string         `json:"edge"`
	Source   string         `json:"source"`
	OriginID string         `json:"origin_id,omitempty"`
	ID       string         `json:"id,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

type hpxMetricRecord struct {
	Table      string         `json:"table"`
	Date       string         `json:"date"`
	Key        string         `json:"key"`
	Value      float64        `json:"value"`
	Source     string         `json:"source"`
	OriginID   string         `json:"origin_id,omitempty"`
	TS         string         `json:"ts,omitempty"`
	SignpostID string         `json:"signpost_id,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
}

type hpxDocumentRecord struct {
	Table      string         `json:"table"`
	Kind       string         `json:"kind"`
	Title      string         `json:"title,omitempty"`
	Body       string         `json:"body,omitempty"`
	Date       string         `json:"date,omitempty"`
	Source     string         `json:"source"`
	OriginID   string         `json:"origin_id,omitempty"`
	ID         string         `json:"id,omitempty"`
	SignpostID string         `json:"signpost_id,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
}

func runHPXExport(cmd *cobra.Command) error {
	opts, err := parseHPXOptions(cmd)
	if err != nil {
		return err
	}

	measureResult, err := measuresListFn(cmd.Context(), &measures.Query{
		Range: cloneListOptions(opts.rangeOpts),
	})
	if err != nil {
		return err
	}

	sleepResult, err := sleepListFn(cmd.Context(), cloneListOptions(opts.rangeOpts))
	if err != nil {
		return err
	}

	workoutResult, err := workoutsListFn(cmd.Context(), cloneListOptions(opts.rangeOpts))
	if err != nil {
		return err
	}

	activityResult, err := activityListFn(cmd.Context(), cloneListOptions(opts.rangeOpts))
	if err != nil {
		return err
	}

	measureGroups := cloneMeasureGroups(measureResult)
	sort.Slice(measureGroups, func(i, j int) bool {
		if measureGroups[i].TakenAt.Equal(measureGroups[j].TakenAt) {
			return measureGroups[i].ID > measureGroups[j].ID
		}
		return measureGroups[i].TakenAt.After(measureGroups[j].TakenAt)
	})
	measureGroups = limitSlice(measureGroups, opts.limit)

	sleepSessions := cloneSleepSessions(sleepResult)
	sort.Slice(sleepSessions, func(i, j int) bool {
		if sleepSessions[i].Start.Equal(sleepSessions[j].Start) {
			return sleepSessions[i].End.After(sleepSessions[j].End)
		}
		return sleepSessions[i].Start.After(sleepSessions[j].Start)
	})
	sleepSessions = limitSlice(sleepSessions, opts.limit)

	workoutItems := cloneWorkouts(workoutResult)
	sort.Slice(workoutItems, func(i, j int) bool {
		if workoutItems[i].Start.Equal(workoutItems[j].Start) {
			return workoutItems[i].End.After(workoutItems[j].End)
		}
		return workoutItems[i].Start.After(workoutItems[j].Start)
	})
	workoutItems = limitSlice(workoutItems, opts.limit)

	activityDays := cloneActivityDays(activityResult)
	sort.Slice(activityDays, func(i, j int) bool {
		if activityDays[i].Date == activityDays[j].Date {
			return activityDays[i].Steps > activityDays[j].Steps
		}
		return activityDays[i].Date > activityDays[j].Date
	})
	activityDays = limitSlice(activityDays, opts.limit)

	signposts := make([]hpxSignpostRecord, 0, len(sleepSessions)*2+len(workoutItems)*2)
	documents := make([]hpxDocumentRecord, 0, len(activityDays))
	metrics := make([]hpxMetricRecord, 0, len(measureGroups)*2+len(sleepSessions)*6+len(workoutItems)*2)

	for _, group := range measureGroups {
		groupDocs, groupMetrics := exportMeasureGroup(group)
		documents = append(documents, groupDocs...)
		metrics = append(metrics, groupMetrics...)
	}

	for _, session := range sleepSessions {
		sessionSignposts, sessionMetrics := exportSleepSession(session)
		signposts = append(signposts, sessionSignposts...)
		metrics = append(metrics, sessionMetrics...)
	}

	for _, workout := range workoutItems {
		workoutSignposts, workoutMetrics := exportWorkout(workout)
		signposts = append(signposts, workoutSignposts...)
		metrics = append(metrics, workoutMetrics...)
	}

	for _, day := range activityDays {
		documents = append(documents, exportActivityDay(day))
	}

	sort.Slice(signposts, func(i, j int) bool {
		if signposts[i].TS == signposts[j].TS {
			if signposts[i].Edge == signposts[j].Edge {
				return signposts[i].ID < signposts[j].ID
			}
			return signpostEdgeOrder(signposts[i].Edge) < signpostEdgeOrder(signposts[j].Edge)
		}
		return signposts[i].TS < signposts[j].TS
	})

	sort.Slice(documents, func(i, j int) bool {
		if documents[i].Date == documents[j].Date {
			return documents[i].OriginID < documents[j].OriginID
		}
		return documents[i].Date < documents[j].Date
	})

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Date == metrics[j].Date {
			if metrics[i].Key == metrics[j].Key {
				if metrics[i].TS == metrics[j].TS {
					return metrics[i].OriginID < metrics[j].OriginID
				}
				return metrics[i].TS < metrics[j].TS
			}
			return metrics[i].Key < metrics[j].Key
		}
		return metrics[i].Date < metrics[j].Date
	})

	return emitHPXRecords(cmd.OutOrStdout(), signposts, documents, metrics)
}

func parseHPXOptions(cmd *cobra.Command) (*hpxOptions, error) {
	sinceValue, err := cmd.Flags().GetString(hpxFlagSince)
	if err != nil {
		return nil, err
	}
	untilValue, err := cmd.Flags().GetString(hpxFlagUntil)
	if err != nil {
		return nil, err
	}
	lastValue, err := cmd.Flags().GetString(hpxFlagLast)
	if err != nil {
		return nil, err
	}
	limit, err := cmd.Flags().GetInt(hpxFlagLimit)
	if err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("%s must be zero or greater", flagName(hpxFlagLimit))
	}
	if strings.TrimSpace(sinceValue) != "" && strings.TrimSpace(lastValue) != "" {
		return nil, fmt.Errorf("%s cannot be combined with %s", flagName(hpxFlagSince), flagName(hpxFlagLast))
	}

	now := hpxNow()

	var start *time.Time
	if strings.TrimSpace(lastValue) != "" {
		parsed, err := parseRelativeWindow(lastValue, now)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", flagName(hpxFlagLast), err)
		}
		start = &parsed
	}
	if strings.TrimSpace(sinceValue) != "" {
		parsed, err := parseHPXBound(sinceValue, false)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", flagName(hpxFlagSince), err)
		}
		start = &parsed
	}

	var end *time.Time
	if strings.TrimSpace(untilValue) != "" {
		parsed, err := parseHPXBound(untilValue, true)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", flagName(hpxFlagUntil), err)
		}
		end = &parsed
	}

	if start == nil {
		value := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		start = &value
	}
	if end == nil {
		value := now.UTC()
		end = &value
	}

	rangeOpts := &api.ListOptions{Start: start, End: end}
	if err := rangeOpts.Validate(); err != nil {
		return nil, err
	}

	return &hpxOptions{
		rangeOpts: rangeOpts,
		limit:     limit,
	}, nil
}

func parseHPXBound(value string, inclusiveEnd bool) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	for _, layout := range timeLayouts {
		if layout == "2006-01-02" {
			t, err := time.ParseInLocation(layout, trimmed, time.Local)
			if err != nil {
				continue
			}
			if inclusiveEnd {
				return t.Add(24 * time.Hour).UTC(), nil
			}
			return t.UTC(), nil
		}
		t, err := time.Parse(layout, trimmed)
		if err != nil {
			continue
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("%q must be RFC3339 or YYYY-MM-DD", value)
}

func parseRelativeWindow(value string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("duration is required")
	}

	if duration, err := time.ParseDuration(trimmed); err == nil {
		return now.Add(-duration).UTC(), nil
	}

	amount, unit, found := splitRelativeWindow(trimmed)
	if !found {
		return time.Time{}, fmt.Errorf("%q must look like 72h, 10d, 2w, 1mo, or 1y", value)
	}
	if amount <= 0 {
		return time.Time{}, fmt.Errorf("%q must be greater than zero", value)
	}

	switch unit {
	case "d":
		return now.Add(-time.Duration(amount) * 24 * time.Hour).UTC(), nil
	case "w":
		return now.Add(-time.Duration(amount*7) * 24 * time.Hour).UTC(), nil
	case "mo":
		return now.AddDate(0, -amount, 0).UTC(), nil
	case "y":
		return now.AddDate(-amount, 0, 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported duration unit %q", unit)
	}
}

func splitRelativeWindow(value string) (int, string, bool) {
	units := []string{"mo", "d", "w", "y"}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit) {
			continue
		}
		amount, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(value, unit)))
		if err != nil {
			return 0, "", false
		}
		return amount, unit, true
	}
	return 0, "", false
}

func exportMeasureGroup(group measures.Group) ([]hpxDocumentRecord, []hpxMetricRecord) {
	ts := group.TakenAt.UTC()
	date := ts.Format("2006-01-02")
	timestamp := ts.Format(time.RFC3339)
	groupID := measureGroupIdentity(group)

	baseMeta := map[string]any{
		"group_id":    group.ID,
		"measured_at": timestamp,
		"category":    group.CategoryName,
	}
	if group.DeviceID != "" {
		baseMeta["device_id"] = group.DeviceID
	}

	extraMeasures := make([]map[string]any, 0, len(group.Measures))
	exported := make([]hpxMetricRecord, 0, 2)
	for _, measure := range group.Measures {
		key, slug := measureMetricKey(measure.Type)
		if key == "" {
			extraMeasures = append(extraMeasures, encodeMeasure(measure))
			continue
		}

		meta := cloneMeta(baseMeta)
		exported = append(exported, hpxMetricRecord{
			Table:    "metric",
			Date:     date,
			Key:      key,
			Value:    measure.Value,
			Source:   hpxSource,
			OriginID: fmt.Sprintf("%s-measure-%s-%s", hpxSource, groupID, slug),
			TS:       timestamp,
			Meta:     meta,
		})
	}

	if len(exported) > 0 {
		if len(extraMeasures) > 0 {
			if exported[0].Meta == nil {
				exported[0].Meta = map[string]any{}
			}
			exported[0].Meta["extra_measures"] = extraMeasures
		}
		return nil, exported
	}

	if len(group.Measures) == 0 {
		return nil, nil
	}

	document := hpxDocumentRecord{
		Table:    "document",
		Kind:     "summary",
		Title:    fmt.Sprintf("Withings body measurements %s", date),
		Body:     "Body measurements imported from Withings.",
		Date:     date,
		Source:   hpxSource,
		OriginID: fmt.Sprintf("%s-measure-%s-summary", hpxSource, groupID),
		Meta: map[string]any{
			"tags":        []string{"withings", "body-measurements"},
			"group_id":    group.ID,
			"measured_at": timestamp,
			"category":    group.CategoryName,
			"measures":    encodeMeasures(group.Measures),
		},
	}
	if group.DeviceID != "" {
		document.Meta["device_id"] = group.DeviceID
	}
	return []hpxDocumentRecord{document}, nil
}

func exportSleepSession(session sleep.Session) ([]hpxSignpostRecord, []hpxMetricRecord) {
	start := session.Start.UTC()
	end := session.End.UTC()
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, nil
	}

	date := session.Date
	if strings.TrimSpace(date) == "" {
		date = start.Format("2006-01-02")
	}
	ts := start.Format(time.RFC3339)
	nativeID := fmt.Sprintf("%d-%d", start.Unix(), end.Unix())
	startID := fmt.Sprintf("%s-sleep-%s-start", hpxSource, nativeID)
	endID := fmt.Sprintf("%s-sleep-%s-end", hpxSource, nativeID)

	signpostData := map[string]any{}
	if session.Date != "" {
		signpostData["source_date"] = session.Date
	}
	if session.Timezone != "" {
		signpostData["timezone"] = session.Timezone
	}
	if session.Model != nil {
		signpostData["model"] = *session.Model
	}
	if session.Modified != nil {
		signpostData["modified_at"] = session.Modified.UTC().Format(time.RFC3339)
	}

	signposts := []hpxSignpostRecord{
		{
			Table:    "signpost",
			Kind:     "sleep",
			TS:       ts,
			Edge:     "start",
			Source:   hpxSource,
			OriginID: startID,
			ID:       startID,
			Data:     normalizeMeta(signpostData),
		},
		{
			Table:    "signpost",
			Kind:     "sleep",
			TS:       end.Format(time.RFC3339),
			Edge:     "end",
			Source:   hpxSource,
			OriginID: endID,
			ID:       endID,
		},
	}

	durationMs := sleepDurationMillis(session, start, end)
	timeInBedMs := durationMillis(start, end)
	durationMeta := map[string]any{
		"start_at": ts,
		"end_at":   end.Format(time.RFC3339),
	}
	if session.Date != "" {
		durationMeta["source_date"] = session.Date
	}
	if session.Timezone != "" {
		durationMeta["timezone"] = session.Timezone
	}
	if session.Model != nil {
		durationMeta["model"] = *session.Model
	}
	if session.Modified != nil {
		durationMeta["modified_at"] = session.Modified.UTC().Format(time.RFC3339)
	}
	if session.Data.SleepScore != nil {
		durationMeta["sleep_score"] = *session.Data.SleepScore
	}
	if session.Data.AverageHeartRate != nil {
		durationMeta["average_heart_rate_bpm"] = *session.Data.AverageHeartRate
	}
	if session.Data.AverageRespRate != nil {
		durationMeta["average_respiratory_rate_rpm"] = *session.Data.AverageRespRate
	}
	if value := secondsToMillis(session.Data.SnoringSeconds); value > 0 {
		durationMeta["snoring_ms"] = value
	}
	if value := secondsToMillis(session.Data.ToSleepSeconds); value > 0 {
		durationMeta["latency_to_sleep_ms"] = value
	}
	if value := secondsToMillis(session.Data.ToWakeSeconds); value > 0 {
		durationMeta["latency_to_wake_ms"] = value
	}

	metrics := make([]hpxMetricRecord, 0, 6)
	if durationMs > 0 {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.duration_ms",
			Value:      float64(durationMs),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-duration", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
			Meta:       normalizeMeta(durationMeta),
		})
	}
	if timeInBedMs > 0 {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.time_in_bed_ms",
			Value:      float64(timeInBedMs),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-time-in-bed", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}
	if durationMs > 0 && timeInBedMs > 0 {
		efficiency := float64(durationMs) * 100 / float64(timeInBedMs)
		if efficiency > 100 {
			efficiency = 100
		}
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.efficiency_pct",
			Value:      efficiency,
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-efficiency", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}
	if value := secondsToMillis(session.Data.REMSleepSeconds); value > 0 {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.rem_ms",
			Value:      float64(value),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-rem", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}
	if value := secondsToMillis(session.Data.LightSleepSeconds); value > 0 {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.light_ms",
			Value:      float64(value),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-light", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}
	if value := secondsToMillis(session.Data.DeepSleepSeconds); value > 0 {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.deep_ms",
			Value:      float64(value),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-deep", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}
	if value := secondsToMillis(session.Data.WakeUpDurationSeconds); value > 0 {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "sleep.awake_ms",
			Value:      float64(value),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-sleep-%s-awake", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}

	return signposts, metrics
}

func exportWorkout(workout workouts.Workout) ([]hpxSignpostRecord, []hpxMetricRecord) {
	start := workout.Start.UTC()
	end := workout.End.UTC()
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, nil
	}

	date := strings.TrimSpace(workout.Date)
	if date == "" {
		date = start.Format("2006-01-02")
	}
	ts := start.Format(time.RFC3339)
	nativeID := fmt.Sprintf("%d-%d", start.Unix(), end.Unix())
	startID := fmt.Sprintf("%s-workout-%s-start", hpxSource, nativeID)
	endID := fmt.Sprintf("%s-workout-%s-end", hpxSource, nativeID)

	signpostData := map[string]any{}
	if workout.Category != nil {
		signpostData["sport_id"] = *workout.Category
	}
	if workout.Timezone != "" {
		signpostData["timezone"] = workout.Timezone
	}
	if workout.Date != "" {
		signpostData["source_date"] = workout.Date
	}
	if workout.Modified != nil {
		signpostData["modified_at"] = workout.Modified.UTC().Format(time.RFC3339)
	}

	signposts := []hpxSignpostRecord{
		{
			Table:    "signpost",
			Kind:     "workout",
			TS:       ts,
			Edge:     "start",
			Source:   hpxSource,
			OriginID: startID,
			ID:       startID,
			Data:     normalizeMeta(signpostData),
		},
		{
			Table:    "signpost",
			Kind:     "workout",
			TS:       end.Format(time.RFC3339),
			Edge:     "end",
			Source:   hpxSource,
			OriginID: endID,
			ID:       endID,
		},
	}

	durationMeta := map[string]any{
		"start_at": ts,
		"end_at":   end.Format(time.RFC3339),
	}
	if workout.Date != "" {
		durationMeta["source_date"] = workout.Date
	}
	if workout.Timezone != "" {
		durationMeta["timezone"] = workout.Timezone
	}
	if workout.Category != nil {
		durationMeta["category"] = *workout.Category
	}
	if workout.Model != nil {
		durationMeta["model"] = *workout.Model
	}
	if workout.Modified != nil {
		durationMeta["modified_at"] = workout.Modified.UTC().Format(time.RFC3339)
	}
	if workout.Attrib != nil {
		durationMeta["attrib"] = *workout.Attrib
	}
	if workout.DeviceID != "" {
		durationMeta["device_id"] = workout.DeviceID
	}
	if workout.Distance != nil {
		durationMeta["distance_m"] = *workout.Distance
	}
	if workout.Elevation != nil {
		durationMeta["elevation_m"] = *workout.Elevation
	}
	if workout.Steps != nil {
		durationMeta["steps"] = *workout.Steps
	}

	metrics := []hpxMetricRecord{
		{
			Table:      "metric",
			Date:       date,
			Key:        "workout.duration_ms",
			Value:      float64(durationMillis(start, end)),
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-workout-%s-duration", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
			Meta:       normalizeMeta(durationMeta),
		},
	}
	if workout.Calories != nil {
		metrics = append(metrics, hpxMetricRecord{
			Table:      "metric",
			Date:       date,
			Key:        "workout.calories_kcal",
			Value:      *workout.Calories,
			Source:     hpxSource,
			OriginID:   fmt.Sprintf("%s-workout-%s-calories", hpxSource, nativeID),
			TS:         ts,
			SignpostID: startID,
		})
	}

	return signposts, metrics
}

func exportActivityDay(day activity.Day) hpxDocumentRecord {
	return hpxDocumentRecord{
		Table:    "document",
		Kind:     "summary",
		Title:    fmt.Sprintf("Withings activity %s", day.Date),
		Body:     "Daily activity summary imported from Withings.",
		Date:     day.Date,
		Source:   hpxSource,
		OriginID: fmt.Sprintf("%s-activity-%s", hpxSource, day.Date),
		Meta: normalizeMeta(map[string]any{
			"tags":                []string{"withings", "activity"},
			"timezone":            emptyAsNil(day.Timezone),
			"steps":               day.Steps,
			"distance_m":          day.Distance,
			"calories_kcal":       day.Calories,
			"total_calories_kcal": day.TotalCalories,
			"elevation_m":         day.Elevation,
			"soft_seconds":        day.Soft,
			"moderate_seconds":    day.Moderate,
			"intense_seconds":     day.Intense,
			"active_seconds":      day.Active,
			"device_id":           emptyAsNil(day.DeviceID),
			"brand":               zeroAsNil(day.Brand),
		}),
	}
}

func emitHPXRecords(
	out io.Writer,
	signposts []hpxSignpostRecord,
	documents []hpxDocumentRecord,
	metrics []hpxMetricRecord,
) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)

	for _, record := range signposts {
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	for _, record := range documents {
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	for _, record := range metrics {
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func measureMetricKey(code int) (string, string) {
	switch code {
	case measures.TypeWeight:
		return "body.weight_kg", "weight"
	case measures.TypeFatRatio:
		return "body.fat_pct", "fat-pct"
	default:
		return "", ""
	}
}

func measureGroupIdentity(group measures.Group) string {
	if group.ID > 0 {
		return strconv.FormatInt(group.ID, 10)
	}
	if group.TakenAt.IsZero() {
		return "unknown"
	}
	return strconv.FormatInt(group.TakenAt.UTC().Unix(), 10)
}

func sleepDurationMillis(session sleep.Session, start, end time.Time) int64 {
	if value := secondsToMillis(session.Data.TimeInBedSeconds); value > 0 {
		return value
	}
	stageTotal := secondsToMillis(session.Data.LightSleepSeconds) +
		secondsToMillis(session.Data.DeepSleepSeconds) +
		secondsToMillis(session.Data.REMSleepSeconds)
	if stageTotal > 0 {
		return stageTotal
	}
	awake := secondsToMillis(session.Data.WakeUpDurationSeconds)
	if span := durationMillis(start, end); span > awake {
		return span - awake
	}
	return durationMillis(start, end)
}

func durationMillis(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}

func secondsToMillis(value *int) int64 {
	if value == nil || *value <= 0 {
		return 0
	}
	return int64(*value) * 1000
}

func encodeMeasures(values []measures.Measure) []map[string]any {
	encoded := make([]map[string]any, 0, len(values))
	for _, value := range values {
		encoded = append(encoded, encodeMeasure(value))
	}
	return encoded
}

func encodeMeasure(value measures.Measure) map[string]any {
	return map[string]any{
		"type":  value.Type,
		"code":  value.Code,
		"name":  value.Name,
		"value": value.Value,
		"unit":  emptyAsNil(value.Unit),
	}
}

func cloneListOptions(opts *api.ListOptions) *api.ListOptions {
	if opts == nil {
		return nil
	}
	copy := *opts
	if opts.Start != nil {
		start := *opts.Start
		copy.Start = &start
	}
	if opts.End != nil {
		end := *opts.End
		copy.End = &end
	}
	return &copy
}

func cloneMeasureGroups(result *measures.ListResult) []measures.Group {
	if result == nil {
		return nil
	}
	return append([]measures.Group(nil), result.Groups...)
}

func cloneSleepSessions(result *sleep.ListResult) []sleep.Session {
	if result == nil {
		return nil
	}
	return append([]sleep.Session(nil), result.Sleeps...)
}

func cloneWorkouts(result *workouts.ListResult) []workouts.Workout {
	if result == nil {
		return nil
	}
	return append([]workouts.Workout(nil), result.Workouts...)
}

func cloneActivityDays(result *activity.ListResult) []activity.Day {
	if result == nil {
		return nil
	}
	return append([]activity.Day(nil), result.Activities...)
}

func cloneMeta(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	copy := make(map[string]any, len(value))
	for key, item := range value {
		copy[key] = item
	}
	return copy
}

func normalizeMeta(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	normalized := make(map[string]any, len(value))
	for key, item := range value {
		switch typed := item.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		case []map[string]any:
			if len(typed) == 0 {
				continue
			}
		case []string:
			if len(typed) == 0 {
				continue
			}
		}
		normalized[key] = item
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func emptyAsNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func zeroAsNil[T ~int | ~int64 | ~float64](value T) any {
	if value == 0 {
		return nil
	}
	return value
}

func flagName(name string) string {
	return "--" + name
}

func signpostEdgeOrder(edge string) int {
	switch edge {
	case "start":
		return 0
	case "end":
		return 1
	default:
		return 2
	}
}

func limitSlice[T any](values []T, limit int) []T {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return append([]T(nil), values[:limit]...)
}
