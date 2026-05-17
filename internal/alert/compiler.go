package alert

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"dash/internal/alertspec"
	"dash/internal/model"
)

type RuleSnapshot struct {
	RuleID          int64   `json:"rule_id"`
	Generation      int64   `json:"generation"`
	Builtin         bool    `json:"builtin,omitempty"`
	Name            string  `json:"name"`
	Metric          string  `json:"metric"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	DurationSec     int32   `json:"duration_sec"`
	CooldownMin     int32   `json:"cooldown_min"`
	ThresholdMode   string  `json:"threshold_mode"`
	ThresholdOffset float64 `json:"threshold_offset"`
	UpdatedAt       string  `json:"updated_at"`
}

type InvalidRule struct {
	RuleID     int64
	Generation int64
	Reason     string
}

type CompiledRule struct {
	RuleID              int64
	Builtin             bool
	Name                string
	Generation          int64
	Metric              string
	Operator            string
	Threshold           float64
	DurationSec         int32
	CooldownMin         int32
	ThresholdMode       string
	ThresholdOffset     float64
	GenerationUpdatedAt time.Time
	Snapshot            RuleSnapshot
}

type CompiledRules struct {
	Rules       []CompiledRule
	ByStateKey  map[string]CompiledRule
	Invalid     []InvalidRule
	RefreshedAt time.Time
}

func CompileRules(items []model.AlertRule, refreshedAt time.Time) *CompiledRules {
	builtins := builtinRules()
	out := &CompiledRules{
		Rules:       make([]CompiledRule, 0, len(items)+len(builtins)),
		ByStateKey:  make(map[string]CompiledRule, len(items)+len(builtins)),
		Invalid:     make([]InvalidRule, 0),
		RefreshedAt: refreshedAt.UTC(),
	}

	for _, rule := range builtins {
		out.add(rule)
	}

	for _, item := range items {
		if item.IsDeleted || !item.Enabled {
			continue
		}
		compiled, invalid := compileRule(item)
		if invalid != nil {
			out.Invalid = append(out.Invalid, *invalid)
			continue
		}
		out.add(*compiled)
	}

	sort.Slice(out.Rules, func(i, j int) bool {
		if out.Rules[i].RuleID == out.Rules[j].RuleID {
			return out.Rules[i].Generation < out.Rules[j].Generation
		}
		return out.Rules[i].RuleID < out.Rules[j].RuleID
	})

	return out
}

func (r *CompiledRules) add(rule CompiledRule) {
	r.Rules = append(r.Rules, rule)
	r.ByStateKey[rule.StateKey()] = rule
}

func (r *CompiledRules) ForMounts(mounts map[int64]bool) *CompiledRules {
	if r == nil {
		return CompileRules(nil, time.Now().UTC())
	}
	out := &CompiledRules{
		Rules:       make([]CompiledRule, 0, len(r.Rules)),
		ByStateKey:  make(map[string]CompiledRule, len(r.Rules)),
		Invalid:     r.Invalid,
		RefreshedAt: r.RefreshedAt,
	}
	for _, rule := range r.Rules {
		out.ByStateKey[rule.StateKey()] = rule
		if !ruleMounted(rule, mounts) {
			continue
		}
		out.Rules = append(out.Rules, rule)
	}
	return out
}

func ruleMounted(rule CompiledRule, mounts map[int64]bool) bool {
	if enabled, ok := mounts[rule.RuleID]; ok {
		return enabled
	}
	return rule.Builtin
}

func (r CompiledRule) StateKey() string {
	return ruleStateKey(r.RuleID, r.Generation)
}

func (r CompiledRule) SnapshotJSON() []byte {
	raw, err := json.Marshal(r.Snapshot)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func compileRule(item model.AlertRule) (*CompiledRule, *InvalidRule) {
	normalized, err := alertspec.NormalizeRuleModel(item)
	if err != nil {
		return nil, invalidRule(item, err)
	}
	updatedAt := normalized.UpdatedAt.UTC()
	snapshot := RuleSnapshot{
		RuleID:          normalized.ID,
		Generation:      normalized.Generation,
		Name:            normalized.Name,
		Metric:          normalized.Metric,
		Operator:        normalized.Operator,
		Threshold:       normalized.Threshold,
		DurationSec:     normalized.DurationSec,
		CooldownMin:     normalized.CooldownMin,
		ThresholdMode:   normalized.ThresholdMode,
		ThresholdOffset: normalized.ThresholdOffset,
		UpdatedAt:       updatedAt.Format(time.RFC3339),
	}
	return &CompiledRule{
		RuleID:              normalized.ID,
		Name:                normalized.Name,
		Generation:          normalized.Generation,
		Metric:              normalized.Metric,
		Operator:            normalized.Operator,
		Threshold:           normalized.Threshold,
		DurationSec:         normalized.DurationSec,
		CooldownMin:         normalized.CooldownMin,
		ThresholdMode:       normalized.ThresholdMode,
		ThresholdOffset:     normalized.ThresholdOffset,
		GenerationUpdatedAt: updatedAt,
		Snapshot:            snapshot,
	}, nil
}

func builtinRules() []CompiledRule {
	updatedAt := time.Unix(0, 0).UTC()
	specs := alertspec.BuiltinRules()
	rules := make([]CompiledRule, 0, len(specs))
	for _, spec := range specs {
		rules = append(rules, builtinRule(spec, updatedAt))
	}
	return rules
}

func builtinRule(spec alertspec.BuiltinRule, updatedAt time.Time) CompiledRule {
	snapshot := RuleSnapshot{
		RuleID:          spec.ID,
		Generation:      1,
		Builtin:         true,
		Name:            spec.Name,
		Metric:          spec.Metric,
		Operator:        spec.Operator,
		Threshold:       spec.Threshold,
		DurationSec:     spec.DurationSec,
		CooldownMin:     spec.CooldownMin,
		ThresholdMode:   "static",
		ThresholdOffset: 0,
		UpdatedAt:       updatedAt.Format(time.RFC3339),
	}
	return CompiledRule{
		RuleID:              spec.ID,
		Builtin:             true,
		Name:                spec.Name,
		Generation:          1,
		Metric:              spec.Metric,
		Operator:            spec.Operator,
		Threshold:           spec.Threshold,
		DurationSec:         spec.DurationSec,
		CooldownMin:         spec.CooldownMin,
		ThresholdMode:       "static",
		ThresholdOffset:     0,
		GenerationUpdatedAt: updatedAt,
		Snapshot:            snapshot,
	}
}

func invalidRule(item model.AlertRule, err error) *InvalidRule {
	return &InvalidRule{
		RuleID:     item.ID,
		Generation: item.Generation,
		Reason:     err.Error(),
	}
}

func ruleStateKey(ruleID, generation int64) string {
	return fmt.Sprintf("%d:%d", ruleID, generation)
}
