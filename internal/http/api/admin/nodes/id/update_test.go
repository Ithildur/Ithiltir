package nodeid

import (
	"encoding/json"
	"errors"
	"testing"

	"dash/internal/nodetags"
	trafficstore "dash/internal/store/traffic"
)

func ptr[T any](value T) *T {
	return &value
}

func TestNormalizeUpdateCycles(t *testing.T) {
	tests := []struct {
		name     string
		in       updateInput
		mode     trafficstore.ServerCycleMode
		day      int
		anchor   string
		timezone string
	}{
		{
			name: "calendar",
			in: updateInput{
				TrafficCycleMode:       ptr(trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)),
				TrafficBillingTimezone: ptr("UTC"),
			},
			mode: trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth), day: 1, timezone: "UTC",
		},
		{
			name: "clamped month end",
			in: updateInput{
				TrafficCycleMode:       ptr(trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd)),
				TrafficBillingStartDay: ptr(20),
				TrafficBillingTimezone: ptr("UTC"),
			},
			mode: trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd), day: 20, timezone: "UTC",
		},
		{
			name: "WHMCS",
			in: updateInput{
				TrafficCycleMode:         ptr(trafficstore.ServerCycleMode(trafficstore.CycleWHMCS)),
				TrafficBillingAnchorDate: ptr("2026-01-30"),
				TrafficBillingTimezone:   ptr("UTC"),
			},
			mode: trafficstore.ServerCycleMode(trafficstore.CycleWHMCS), day: 30, anchor: "2026-01-30", timezone: "UTC",
		},
		{
			name: "legacy default",
			in: updateInput{
				TrafficCycleMode: ptr(trafficstore.ServerCycleMode(trafficstore.ServerCycleDefault)),
			},
			mode: trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth), day: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			if err := normalizeUpdate(&in); err != nil {
				t.Fatalf("normalizeUpdate() error = %v", err)
			}
			requireUpdateValue(t, "cycle mode", in.TrafficCycleMode, tt.mode)
			requireUpdateValue(t, "billing start day", in.TrafficBillingStartDay, tt.day)
			requireUpdateValue(t, "billing anchor", in.TrafficBillingAnchorDate, tt.anchor)
			requireUpdateValue(t, "billing timezone", in.TrafficBillingTimezone, tt.timezone)
		})
	}
}

func TestNormalizeUpdateRejectsInvalidTrafficSettings(t *testing.T) {
	tests := []struct {
		name string
		in   updateInput
		want error
	}{
		{
			name: "incomplete cycle",
			in: updateInput{
				TrafficCycleMode: ptr(trafficstore.ServerCycleMode(trafficstore.CycleClampMonthEnd)),
			},
			want: errIncompleteTrafficCycleSettings,
		},
		{
			name: "empty WHMCS anchor",
			in: updateInput{
				TrafficCycleMode:         ptr(trafficstore.ServerCycleMode(trafficstore.CycleWHMCS)),
				TrafficBillingAnchorDate: ptr(""),
				TrafficBillingTimezone:   ptr("UTC"),
			},
			want: errInvalidTrafficBillingAnchor,
		},
		{
			name: "WHMCS start day",
			in: updateInput{
				TrafficCycleMode:         ptr(trafficstore.ServerCycleMode(trafficstore.CycleWHMCS)),
				TrafficBillingStartDay:   ptr(30),
				TrafficBillingAnchorDate: ptr("2026-01-30"),
				TrafficBillingTimezone:   ptr("UTC"),
			},
			want: errIncompleteTrafficCycleSettings,
		},
		{
			name: "calendar start day",
			in: updateInput{
				TrafficCycleMode:       ptr(trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)),
				TrafficBillingStartDay: ptr(30),
				TrafficBillingTimezone: ptr("UTC"),
			},
			want: errIncompleteTrafficCycleSettings,
		},
		{
			name: "legacy default timezone",
			in: updateInput{
				TrafficCycleMode:       ptr(trafficstore.ServerCycleMode(trafficstore.ServerCycleDefault)),
				TrafficBillingTimezone: ptr("UTC"),
			},
			want: errIncompleteTrafficCycleSettings,
		},
		{
			name: "unknown timezone",
			in: updateInput{
				TrafficCycleMode:       ptr(trafficstore.ServerCycleMode(trafficstore.CycleCalendarMonth)),
				TrafficBillingTimezone: ptr("No/Such_Zone"),
			},
			want: errInvalidTrafficBillingTimezone,
		},
		{
			name: "empty direction",
			in: updateInput{
				TrafficDirectionMode: ptr(trafficstore.ServerDirectionMode("")),
			},
			want: errInvalidTrafficDirectionMode,
		},
		{
			name: "unknown direction",
			in: updateInput{
				TrafficDirectionMode: ptr(trafficstore.ServerDirectionMode("in")),
			},
			want: errInvalidTrafficDirectionMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			if err := normalizeUpdate(&in); !errors.Is(err, tt.want) {
				t.Fatalf("normalizeUpdate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNormalizeUpdateTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		err  error
	}{
		{name: "normalize", raw: `[" edge ","db","","edge"]`, want: `["edge","db"]`},
		{name: "clear", raw: `[]`, want: `[]`},
		{name: "null", raw: `null`, err: nodetags.ErrInvalid},
		{name: "object", raw: `{"role":"db"}`, err: nodetags.ErrInvalid},
		{name: "non-string item", raw: `[1]`, err: nodetags.ErrInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := updateInput{Tags: json.RawMessage(tt.raw)}
			err := normalizeUpdate(&in)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("normalizeUpdate() error = %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeUpdate() error = %v", err)
			}
			if got := string(in.Tags); got != tt.want {
				t.Fatalf("tags = %s, want %s", got, tt.want)
			}
			upd := updateFromInput(in)
			if upd.Tags == nil || string(*upd.Tags) != tt.want {
				t.Fatalf("update tags = %v, want %s", upd.Tags, tt.want)
			}
		})
	}
}

func requireUpdateValue[T comparable](t *testing.T, name string, got *T, want T) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
