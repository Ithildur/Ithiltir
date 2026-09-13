package rules

import (
	"errors"
	"strings"

	"dash/internal/model"
	alertstore "dash/internal/store/alert"
)

func ruleFromInput(in createInput) (*model.AlertRule, error) {
	if in.Threshold == nil {
		return nil, errors.New("threshold is required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return &model.AlertRule{
		Name:            in.Name,
		Enabled:         enabled,
		Metric:          in.Metric,
		Operator:        in.Operator,
		Threshold:       *in.Threshold,
		DurationSec:     durationSec(in.DurationSec),
		CooldownMin:     derefInt32(in.CooldownMin),
		ThresholdMode:   derefString(in.ThresholdMode),
		ThresholdOffset: derefFloat64(in.ThresholdOffset),
	}, nil
}

func patchFromInput(in updateInput) (alertstore.AlertRulePatch, bool, error) {
	patch := alertstore.AlertRulePatch{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return alertstore.AlertRulePatch{}, false, errors.New("name cannot be empty")
		}
		patch.Name = &name
	}
	if in.Enabled != nil {
		patch.Enabled = in.Enabled
	}
	if in.Metric != nil {
		metric := strings.TrimSpace(*in.Metric)
		if metric == "" {
			return alertstore.AlertRulePatch{}, false, errors.New("metric cannot be empty")
		}
		patch.Metric = &metric
	}
	if in.Operator != nil {
		operator := strings.TrimSpace(*in.Operator)
		if operator == "" {
			return alertstore.AlertRulePatch{}, false, errors.New("operator cannot be empty")
		}
		patch.Operator = &operator
	}
	if in.Threshold != nil {
		patch.Threshold = in.Threshold
	}
	if in.DurationSec != nil {
		patch.DurationSec = in.DurationSec
	}
	if in.CooldownMin != nil {
		patch.CooldownMin = in.CooldownMin
	}
	if in.ThresholdMode != nil {
		mode := strings.TrimSpace(*in.ThresholdMode)
		if mode == "" {
			return alertstore.AlertRulePatch{}, false, errors.New("threshold_mode cannot be empty")
		}
		patch.ThresholdMode = &mode
	}
	if in.ThresholdOffset != nil {
		patch.ThresholdOffset = in.ThresholdOffset
	}
	return patch, patch.HasUpdates(), nil
}

func derefFloat64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func durationSec(v *int32) int32 {
	if v == nil {
		return 60
	}
	return *v
}

func derefInt32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}
