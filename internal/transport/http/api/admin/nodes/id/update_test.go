package nodeid

import (
	"encoding/json"
	"testing"

	"dash/internal/nodetags"
	trafficstore "dash/internal/store/traffic"
)

func TestNormalizeUpdateRejectsIncompleteCycleSettings(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd)
	in := updateInput{TrafficCycleMode: &mode}

	if err := normalizeUpdate(&in); err != errIncompleteTrafficCycleSettings {
		t.Fatalf("normalizeUpdate() error = %v, want %v", err, errIncompleteTrafficCycleSettings)
	}
}

func TestNormalizeUpdateNormalizesCalendarCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)
	timezone := "UTC"
	in := updateInput{
		TrafficCycleMode:       &mode,
		TrafficBillingTimezone: &timezone,
	}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if in.TrafficBillingStartDay == nil || *in.TrafficBillingStartDay != 1 {
		t.Fatalf("billing start day = %v, want 1", in.TrafficBillingStartDay)
	}
	if in.TrafficBillingAnchorDate == nil || *in.TrafficBillingAnchorDate != "" {
		t.Fatalf("billing anchor = %v, want empty string", in.TrafficBillingAnchorDate)
	}
	if in.TrafficBillingTimezone == nil || *in.TrafficBillingTimezone != "UTC" {
		t.Fatalf("billing timezone = %v, want UTC", in.TrafficBillingTimezone)
	}
}

func TestNormalizeUpdateNormalizesClampCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd)
	day := 20
	timezone := "UTC"
	in := updateInput{
		TrafficCycleMode:       &mode,
		TrafficBillingStartDay: &day,
		TrafficBillingTimezone: &timezone,
	}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if in.TrafficBillingStartDay == nil || *in.TrafficBillingStartDay != 20 {
		t.Fatalf("billing start day = %v, want 20", in.TrafficBillingStartDay)
	}
	if in.TrafficBillingAnchorDate == nil || *in.TrafficBillingAnchorDate != "" {
		t.Fatalf("billing anchor = %v, want empty string", in.TrafficBillingAnchorDate)
	}
	if in.TrafficBillingTimezone == nil || *in.TrafficBillingTimezone != "UTC" {
		t.Fatalf("billing timezone = %v, want UTC", in.TrafficBillingTimezone)
	}
}

func TestNormalizeUpdateNormalizesWHMCSCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleWHMCS)
	anchor := "2026-01-30"
	timezone := "UTC"
	in := updateInput{
		TrafficCycleMode:         &mode,
		TrafficBillingAnchorDate: &anchor,
		TrafficBillingTimezone:   &timezone,
	}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if in.TrafficBillingStartDay == nil || *in.TrafficBillingStartDay != 30 {
		t.Fatalf("billing start day = %v, want 30", in.TrafficBillingStartDay)
	}
	if in.TrafficBillingAnchorDate == nil || *in.TrafficBillingAnchorDate != "2026-01-30" {
		t.Fatalf("billing anchor = %v, want 2026-01-30", in.TrafficBillingAnchorDate)
	}
	if in.TrafficBillingTimezone == nil || *in.TrafficBillingTimezone != "UTC" {
		t.Fatalf("billing timezone = %v, want UTC", in.TrafficBillingTimezone)
	}
}

func TestNormalizeUpdateRejectsEmptyWHMCSAnchor(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleWHMCS)
	anchor := ""
	timezone := "UTC"
	in := updateInput{
		TrafficCycleMode:         &mode,
		TrafficBillingAnchorDate: &anchor,
		TrafficBillingTimezone:   &timezone,
	}

	if err := normalizeUpdate(&in); err != errInvalidTrafficBillingAnchor {
		t.Fatalf("normalizeUpdate(empty whmcs anchor) error = %v, want %v", err, errInvalidTrafficBillingAnchor)
	}
}

func TestNormalizeUpdateClearsDefaultCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleDefault
	in := updateInput{
		TrafficCycleMode: &mode,
	}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if in.TrafficBillingStartDay == nil || *in.TrafficBillingStartDay != 1 {
		t.Fatalf("billing start day = %v, want 1", in.TrafficBillingStartDay)
	}
	if in.TrafficBillingAnchorDate == nil || *in.TrafficBillingAnchorDate != "" {
		t.Fatalf("billing anchor = %v, want empty string", in.TrafficBillingAnchorDate)
	}
	if in.TrafficBillingTimezone == nil || *in.TrafficBillingTimezone != "" {
		t.Fatalf("billing timezone = %v, want empty string", in.TrafficBillingTimezone)
	}
}

func TestNormalizeUpdateRejectsUnusedCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleWHMCS)
	day := 30
	anchor := "2026-01-30"
	timezone := "UTC"
	in := updateInput{
		TrafficCycleMode:         &mode,
		TrafficBillingStartDay:   &day,
		TrafficBillingAnchorDate: &anchor,
		TrafficBillingTimezone:   &timezone,
	}
	if err := normalizeUpdate(&in); err != errIncompleteTrafficCycleSettings {
		t.Fatalf("normalizeUpdate(whmcs with start day) error = %v, want %v", err, errIncompleteTrafficCycleSettings)
	}

	mode = trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)
	in = updateInput{
		TrafficCycleMode:       &mode,
		TrafficBillingStartDay: &day,
		TrafficBillingTimezone: &timezone,
	}
	if err := normalizeUpdate(&in); err != errIncompleteTrafficCycleSettings {
		t.Fatalf("normalizeUpdate(calendar with start day) error = %v, want %v", err, errIncompleteTrafficCycleSettings)
	}

	mode = trafficstore.ServerCycleDefault
	in = updateInput{
		TrafficCycleMode:       &mode,
		TrafficBillingTimezone: &timezone,
	}
	if err := normalizeUpdate(&in); err != errIncompleteTrafficCycleSettings {
		t.Fatalf("normalizeUpdate(default with timezone) error = %v, want %v", err, errIncompleteTrafficCycleSettings)
	}
}

func TestNormalizeUpdateRejectsInvalidRelevantTimezone(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)
	timezone := "No/Such_Zone"
	in := updateInput{
		TrafficCycleMode:       &mode,
		TrafficBillingTimezone: &timezone,
	}

	if err := normalizeUpdate(&in); err != errInvalidTrafficBillingTimezone {
		t.Fatalf("normalizeUpdate() error = %v, want %v", err, errInvalidTrafficBillingTimezone)
	}
}

func TestNormalizeUpdateNormalizesDirectionMode(t *testing.T) {
	mode := trafficstore.ServerDirectionMode("")
	in := updateInput{TrafficDirectionMode: &mode}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if in.TrafficDirectionMode == nil || *in.TrafficDirectionMode != trafficstore.ServerDirectionDefault {
		t.Fatalf("direction mode = %v, want %q", in.TrafficDirectionMode, trafficstore.ServerDirectionDefault)
	}

	invalid := trafficstore.ServerDirectionMode("in")
	in = updateInput{TrafficDirectionMode: &invalid}
	if err := normalizeUpdate(&in); err != errInvalidTrafficDirectionMode {
		t.Fatalf("normalizeUpdate(invalid direction) error = %v, want %v", err, errInvalidTrafficDirectionMode)
	}
}

func TestNormalizeUpdateNormalizesTags(t *testing.T) {
	in := updateInput{Tags: json.RawMessage(`[" edge ","db","","edge"]`)}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if got, want := string(in.Tags), `["edge","db"]`; got != want {
		t.Fatalf("tags = %s, want %s", got, want)
	}
	upd := updateFromInput(in)
	if upd.Tags == nil || string(*upd.Tags) != `["edge","db"]` {
		t.Fatalf("update tags = %v, want normalized tags", upd.Tags)
	}
}

func TestNormalizeUpdateClearsTags(t *testing.T) {
	in := updateInput{Tags: json.RawMessage(`[]`)}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if got, want := string(in.Tags), `[]`; got != want {
		t.Fatalf("tags = %s, want %s", got, want)
	}
}

func TestNormalizeUpdateRejectsNullTags(t *testing.T) {
	in := updateInput{Tags: json.RawMessage(`null`)}

	if err := normalizeUpdate(&in); err != nodetags.ErrInvalid {
		t.Fatalf("normalizeUpdate() error = %v, want %v", err, nodetags.ErrInvalid)
	}
}

func TestNormalizeUpdateRejectsNonStringArrayTags(t *testing.T) {
	for _, raw := range []string{`{"role":"db"}`, `[1]`} {
		in := updateInput{Tags: json.RawMessage(raw)}
		if err := normalizeUpdate(&in); err != nodetags.ErrInvalid {
			t.Fatalf("normalizeUpdate(%s) error = %v, want %v", raw, err, nodetags.ErrInvalid)
		}
	}
}
