package measures

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/toto/withingy/internal/api"
)

const measurePath = "/measure"

const (
	CategoryReal      = 1
	CategoryObjective = 2
)

const (
	TypeWeight                 = 1
	TypeHeight                 = 4
	TypeFatFreeMass            = 5
	TypeFatRatio               = 6
	TypeFatMassWeight          = 8
	TypeDiastolicBloodPressure = 9
	TypeSystolicBloodPressure  = 10
	TypeHeartPulse             = 11
	TypeTemperature            = 12
	TypeSpO2                   = 54
	TypeBodyTemperature        = 71
	TypeSkinTemperature        = 73
	TypeMuscleMass             = 76
	TypeHydration              = 77
	TypeBoneMass               = 88
	TypePulseWaveVelocity      = 91
)

type measureTypeInfo struct {
	Key   string
	Label string
	Unit  string
}

var measureTypeInfoByCode = map[int]measureTypeInfo{
	TypeWeight:                 {Key: "weight", Label: "Weight", Unit: "kg"},
	TypeHeight:                 {Key: "height", Label: "Height", Unit: "m"},
	TypeFatFreeMass:            {Key: "fat-free-mass", Label: "Fat Free Mass", Unit: "kg"},
	TypeFatRatio:               {Key: "fat-ratio", Label: "Fat Ratio", Unit: "%"},
	TypeFatMassWeight:          {Key: "fat-mass", Label: "Fat Mass", Unit: "kg"},
	TypeDiastolicBloodPressure: {Key: "diastolic-blood-pressure", Label: "Diastolic Blood Pressure", Unit: "mmHg"},
	TypeSystolicBloodPressure:  {Key: "systolic-blood-pressure", Label: "Systolic Blood Pressure", Unit: "mmHg"},
	TypeHeartPulse:             {Key: "heart-pulse", Label: "Heart Pulse", Unit: "bpm"},
	TypeTemperature:            {Key: "temperature", Label: "Temperature", Unit: "C"},
	TypeSpO2:                   {Key: "spo2", Label: "SpO2", Unit: "%"},
	TypeBodyTemperature:        {Key: "body-temperature", Label: "Body Temperature", Unit: "C"},
	TypeSkinTemperature:        {Key: "skin-temperature", Label: "Skin Temperature", Unit: "C"},
	TypeMuscleMass:             {Key: "muscle-mass", Label: "Muscle Mass", Unit: "kg"},
	TypeHydration:              {Key: "hydration", Label: "Hydration", Unit: "kg"},
	TypeBoneMass:               {Key: "bone-mass", Label: "Bone Mass", Unit: "kg"},
	TypePulseWaveVelocity:      {Key: "pulse-wave-velocity", Label: "Pulse Wave Velocity", Unit: "m/s"},
}

var measureTypeCodeByAlias = map[string]int{
	"bone-mass":                TypeBoneMass,
	"body-temperature":         TypeBodyTemperature,
	"diastolic-blood-pressure": TypeDiastolicBloodPressure,
	"fat-free-mass":            TypeFatFreeMass,
	"fat-mass":                 TypeFatMassWeight,
	"fat-mass-weight":          TypeFatMassWeight,
	"fat-ratio":                TypeFatRatio,
	"heart-pulse":              TypeHeartPulse,
	"heart-rate":               TypeHeartPulse,
	"height":                   TypeHeight,
	"hydration":                TypeHydration,
	"muscle-mass":              TypeMuscleMass,
	"pulse-wave-velocity":      TypePulseWaveVelocity,
	"pwv":                      TypePulseWaveVelocity,
	"skin-temperature":         TypeSkinTemperature,
	"spo2":                     TypeSpO2,
	"spo-2":                    TypeSpO2,
	"systolic-blood-pressure":  TypeSystolicBloodPressure,
	"temperature":              TypeTemperature,
	"weight":                   TypeWeight,
}

// Service fetches Withings body measurements.
type Service struct {
	client interface {
		PostFormJSON(ctx context.Context, path string, form url.Values, dest any) error
	}
	now func() time.Time
}

// NewService constructs a measures Service backed by the shared API client.
func NewService(client *api.Client) *Service {
	return &Service{client: client, now: time.Now}
}

// Query mirrors the key Getmeas filter knobs.
type Query struct {
	Range      *api.ListOptions
	Types      []int
	Category   *int
	LastUpdate *int64
}

// ListResult captures decoded measure groups.
type ListResult struct {
	Groups []Group `json:"groups"`
	More   bool    `json:"more,omitempty"`
	Offset int64   `json:"offset,omitempty"`
}

// Group is a single measurement group from Withings.
type Group struct {
	ID           int64     `json:"id"`
	TakenAt      time.Time `json:"taken_at"`
	Category     int       `json:"category"`
	CategoryName string    `json:"category_name,omitempty"`
	Attributes   int       `json:"attributes,omitempty"`
	DeviceID     string    `json:"device_id,omitempty"`
	Measures     []Measure `json:"measures"`
}

// Measure is one normalized measurement value.
type Measure struct {
	Type     int     `json:"type"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit,omitempty"`
	RawValue int64   `json:"raw_value"`
	RawUnit  int     `json:"raw_unit"`
}

// WeightEntry is a weight-specific projection over a measurement group.
type WeightEntry struct {
	GroupID  int64     `json:"group_id"`
	TakenAt  time.Time `json:"taken_at"`
	WeightKG float64   `json:"weight_kg"`
}

// WeightListResult captures projected weight entries.
type WeightListResult struct {
	Weights []WeightEntry `json:"weights"`
}

// ParseTypes accepts a comma-separated list of Withings measure names or integer codes.
func ParseTypes(value string) ([]int, error) {
	fields := strings.Split(value, ",")
	types := make([]int, 0, len(fields))
	seen := make(map[int]struct{}, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		parsed, err := ParseType(trimmed)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[parsed]; ok {
			continue
		}
		seen[parsed] = struct{}{}
		types = append(types, parsed)
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("types must not be empty")
	}
	return types, nil
}

// ParseType accepts a Withings measure type alias or numeric code.
func ParseType(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("measure type is required")
	}
	if code, err := strconv.Atoi(trimmed); err == nil {
		if code <= 0 {
			return 0, fmt.Errorf("measure type code must be greater than zero")
		}
		return code, nil
	}

	key := normalizeTypeKey(trimmed)
	if code, ok := measureTypeCodeByAlias[key]; ok {
		return code, nil
	}
	return 0, fmt.Errorf("unsupported measure type %q", value)
}

// TypeKey returns a stable CLI-friendly key for the measure type.
func TypeKey(code int) string {
	info, ok := measureTypeInfoByCode[code]
	if !ok {
		return fmt.Sprintf("type-%d", code)
	}
	return info.Key
}

// TypeLabel returns a human-readable label for the measure type.
func TypeLabel(code int) string {
	info, ok := measureTypeInfoByCode[code]
	if !ok {
		return fmt.Sprintf("Type %d", code)
	}
	return info.Label
}

// TypeUnit returns the natural display unit for the measure type when known.
func TypeUnit(code int) string {
	info, ok := measureTypeInfoByCode[code]
	if !ok {
		return ""
	}
	return info.Unit
}

// CategoryLabel returns a human-readable name for a Withings measure category.
func CategoryLabel(category int) string {
	switch category {
	case CategoryReal:
		return "real"
	case CategoryObjective:
		return "objective"
	default:
		return fmt.Sprintf("unknown(%d)", category)
	}
}

// List retrieves Withings measurement groups with optional filters.
func (s *Service) List(ctx context.Context, query *Query) (*ListResult, error) {
	if query == nil {
		query = &Query{}
	}
	if err := query.validate(); err != nil {
		return nil, err
	}

	start, end := query.rangeBounds(s.now())
	form := url.Values{}
	form.Set("action", "getmeas")
	if start != nil {
		form.Set("startdate", strconv.FormatInt(start.UTC().Unix(), 10))
	}
	if end != nil {
		form.Set("enddate", strconv.FormatInt(end.UTC().Unix(), 10))
	}
	if query.Category != nil {
		form.Set("category", strconv.Itoa(*query.Category))
	}
	if query.LastUpdate != nil {
		form.Set("lastupdate", strconv.FormatInt(*query.LastUpdate, 10))
	}
	if len(query.Types) > 0 {
		form.Set("meastype", joinTypeCodes(query.Types))
	}

	var body struct {
		MeasureGroups []measureGroupRecord `json:"measuregrps"`
		More          int                  `json:"more"`
		Offset        int64                `json:"offset"`
	}
	if err := s.client.PostFormJSON(ctx, measurePath, form, &body); err != nil {
		return nil, fmt.Errorf("fetch measurements: %w", err)
	}

	allowedTypes := make(map[int]struct{}, len(query.Types))
	for _, code := range query.Types {
		allowedTypes[code] = struct{}{}
	}

	groups := make([]Group, 0, len(body.MeasureGroups))
	for _, record := range body.MeasureGroups {
		group := convertGroup(record)
		if start != nil && group.TakenAt.Before(start.UTC()) {
			continue
		}
		if end != nil && !group.TakenAt.Before(end.UTC()) {
			continue
		}
		if query.Category != nil && group.Category != *query.Category {
			continue
		}
		if len(allowedTypes) > 0 {
			filtered := make([]Measure, 0, len(group.Measures))
			for _, measure := range group.Measures {
				if _, ok := allowedTypes[measure.Type]; ok {
					filtered = append(filtered, measure)
				}
			}
			if len(filtered) == 0 {
				continue
			}
			group.Measures = filtered
		}
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TakenAt.Equal(groups[j].TakenAt) {
			return groups[i].ID > groups[j].ID
		}
		return groups[i].TakenAt.After(groups[j].TakenAt)
	})

	return &ListResult{
		Groups: groups,
		More:   body.More != 0,
		Offset: body.Offset,
	}, nil
}

// WeightList retrieves weight measurement groups and projects them into a simple list.
func (s *Service) WeightList(ctx context.Context, opts *api.ListOptions) (*WeightListResult, error) {
	category := CategoryReal
	result, err := s.List(ctx, &Query{
		Range:    opts,
		Types:    []int{TypeWeight},
		Category: &category,
	})
	if err != nil {
		return nil, err
	}

	weights := make([]WeightEntry, 0, len(result.Groups))
	for _, group := range result.Groups {
		measure := measureByType(group.Measures, TypeWeight)
		if measure == nil {
			continue
		}
		weights = append(weights, WeightEntry{
			GroupID:  group.ID,
			TakenAt:  group.TakenAt,
			WeightKG: roundFloat(measure.Value, 2),
		})
	}
	return &WeightListResult{Weights: weights}, nil
}

// LatestWeight returns the most recent real weight measurement in the account history.
func (s *Service) LatestWeight(ctx context.Context) (*WeightEntry, error) {
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	end := s.now().UTC().Add(24 * time.Hour)
	result, err := s.WeightList(ctx, &api.ListOptions{Start: &start, End: &end})
	if err != nil {
		return nil, err
	}
	if len(result.Weights) == 0 {
		return nil, nil
	}
	latest := result.Weights[0]
	return &latest, nil
}

func (q *Query) validate() error {
	if q.Range != nil {
		if err := q.Range.Validate(); err != nil {
			return err
		}
	}
	if q.Category != nil && *q.Category <= 0 {
		return fmt.Errorf("category must be greater than zero")
	}
	if q.LastUpdate != nil && *q.LastUpdate < 0 {
		return fmt.Errorf("last update must be zero or greater")
	}
	for _, code := range q.Types {
		if code <= 0 {
			return fmt.Errorf("measure type codes must be greater than zero")
		}
	}
	return nil
}

func (q *Query) rangeBounds(now time.Time) (*time.Time, *time.Time) {
	var start *time.Time
	var end *time.Time
	if q.Range != nil {
		start = q.Range.Start
		end = q.Range.End
	}
	if start == nil && end == nil && q.LastUpdate == nil {
		defaults := defaultMeasureOptions(now)
		start = defaults.Start
		end = defaults.End
	}
	return start, end
}

func convertGroup(record measureGroupRecord) Group {
	group := Group{
		ID:           record.GroupID,
		TakenAt:      time.Unix(record.Date, 0).UTC(),
		Category:     record.Category,
		CategoryName: CategoryLabel(record.Category),
		Attributes:   record.Attrib,
		DeviceID:     record.DeviceID,
		Measures:     make([]Measure, 0, len(record.Measures)),
	}
	for _, measure := range record.Measures {
		group.Measures = append(group.Measures, convertMeasure(measure))
	}
	return group
}

func convertMeasure(record measureRecord) Measure {
	return Measure{
		Type:     record.Type,
		Code:     TypeKey(record.Type),
		Name:     TypeLabel(record.Type),
		Value:    float64(record.Value) * math.Pow10(record.Unit),
		Unit:     TypeUnit(record.Type),
		RawValue: record.Value,
		RawUnit:  record.Unit,
	}
}

func defaultMeasureOptions(now time.Time) *api.ListOptions {
	end := now.UTC().Add(24 * time.Hour)
	start := end.Add(-30 * 24 * time.Hour)
	return &api.ListOptions{Start: &start, End: &end}
}

func measureByType(measures []Measure, wanted int) *Measure {
	for i := range measures {
		if measures[i].Type == wanted {
			return &measures[i]
		}
	}
	return nil
}

func joinTypeCodes(types []int) string {
	values := make([]string, 0, len(types))
	for _, code := range types {
		values = append(values, strconv.Itoa(code))
	}
	return strings.Join(values, ",")
}

func normalizeTypeKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	return normalized
}

func roundFloat(value float64, places int) float64 {
	if places < 0 {
		return value
	}
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

type measureGroupRecord struct {
	GroupID  int64           `json:"grpid"`
	Attrib   int             `json:"attrib"`
	Date     int64           `json:"date"`
	Category int             `json:"category"`
	DeviceID string          `json:"deviceid"`
	Measures []measureRecord `json:"measures"`
}

type measureRecord struct {
	Value int64 `json:"value"`
	Type  int   `json:"type"`
	Unit  int   `json:"unit"`
}
