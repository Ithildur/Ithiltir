package nodes

import (
	"encoding/json"
	"testing"

	trafficstore "dash/internal/store/traffic"
)

func TestNormalizeUpdateKeepsUnspecifiedCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd)
	in := updateInput{TrafficCycleMode: &mode}

	if err := normalizeUpdate(&in); err != nil {
		t.Fatalf("normalizeUpdate() error = %v", err)
	}
	if in.TrafficCycleMode == nil || *in.TrafficCycleMode != mode {
		t.Fatalf("traffic cycle mode = %v, want %q", in.TrafficCycleMode, mode)
	}
	if in.TrafficBillingStartDay != nil {
		t.Fatalf("billing start day = %v, want nil", *in.TrafficBillingStartDay)
	}
	if in.TrafficBillingAnchorDate != nil {
		t.Fatalf("billing anchor = %q, want nil", *in.TrafficBillingAnchorDate)
	}
	if in.TrafficBillingTimezone != nil {
		t.Fatalf("billing timezone = %q, want nil", *in.TrafficBillingTimezone)
	}
}

func TestNormalizeUpdateNormalizesSubmittedCycleFields(t *testing.T) {
	mode := trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)
	day := 20
	anchor := "2026-01-30"
	in := updateInput{
		TrafficCycleMode:         &mode,
		TrafficBillingStartDay:   &day,
		TrafficBillingAnchorDate: &anchor,
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
	if in.TrafficBillingTimezone != nil {
		t.Fatalf("billing timezone = %q, want nil", *in.TrafficBillingTimezone)
	}
}

func TestNormalizeUpdateRejectsInvalidSubmittedFieldWithDefaultMode(t *testing.T) {
	mode := trafficstore.ServerCycleDefault
	timezone := "No/Such_Zone"
	in := updateInput{
		TrafficCycleMode:       &mode,
		TrafficBillingTimezone: &timezone,
	}

	if err := normalizeUpdate(&in); err != errInvalidTrafficBillingTimezone {
		t.Fatalf("normalizeUpdate() error = %v, want %v", err, errInvalidTrafficBillingTimezone)
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

	if err := normalizeUpdate(&in); err != errInvalidNodeTags {
		t.Fatalf("normalizeUpdate() error = %v, want %v", err, errInvalidNodeTags)
	}
}

func TestNormalizeUpdateRejectsNonStringArrayTags(t *testing.T) {
	for _, raw := range []string{`{"role":"db"}`, `[1]`} {
		in := updateInput{Tags: json.RawMessage(raw)}
		if err := normalizeUpdate(&in); err != errInvalidNodeTags {
			t.Fatalf("normalizeUpdate(%s) error = %v, want %v", raw, err, errInvalidNodeTags)
		}
	}
}
