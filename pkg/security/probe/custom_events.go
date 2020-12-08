// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

// +build linux

package probe

import (
	"encoding/json"
	"time"

	"github.com/DataDog/datadog-agent/pkg/security/rules"
	"github.com/DataDog/datadog-agent/pkg/security/secl/eval"
)

const (
	// LostEventsRuleID is the rule ID for the lost_events_* events
	LostEventsRuleID = "lost_events"
	// RuleSetLoadedRuleID is the rule ID for the ruleset_loaded events
	RuleSetLoadedRuleID = "ruleset_loaded"
	// NoisyProcessRuleID is the rule ID for the noisy_process events
	NoisyProcessRuleID = "noisy_process"
	// AbnormalPathRuleID is the rule ID for the abnormal_path events
	AbnormalPathRuleID = "abnormal_path"
	// ForkBombRuleID is the rule ID for the fork_bomb events
	ForkBombRuleID = "fork_bomb"
)

// AllCustomRuleIDs returns the list of custom rule IDs
func AllCustomRuleIDs() []string {
	return []string{
		LostEventsRuleID,
		RuleSetLoadedRuleID,
		NoisyProcessRuleID,
		AbnormalPathRuleID,
		ForkBombRuleID,
	}
}

// CustomEvent is used to send custom security events to Datadog
type CustomEvent struct {
	eventType   string
	tags        []string
	marshalFunc func() ([]byte, error)
}

// Clone returns a copy of the current CustomEvent
func (ce *CustomEvent) Clone() CustomEvent {
	return CustomEvent{
		eventType:   ce.eventType,
		tags:        ce.tags,
		marshalFunc: ce.marshalFunc,
	}
}

// GetTags returns the tags of the custom event
func (ce *CustomEvent) GetTags() []string {
	return append(ce.tags, "type:"+ce.GetType())
}

// GetType returns the type of the custom event
func (ce *CustomEvent) GetType() string {
	return ce.eventType
}

// MarshalJSON is the JSON marshaller function of the custom event
func (ce *CustomEvent) MarshalJSON() ([]byte, error) {
	if ce.marshalFunc != nil {
		return ce.marshalFunc()
	}
	return nil, nil
}

// String returns the string representation of a custom event
func (ce *CustomEvent) String() string {
	d, err := json.Marshal(ce)
	if err != nil {
		return err.Error()
	}
	return string(d)
}

func fillCustomRule(ruleDef *rules.RuleDefinition, event *CustomEvent) (*rules.Rule, *CustomEvent) {
	return &rules.Rule{
		Rule:       &eval.Rule{ID: ruleDef.ID},
		Definition: ruleDef,
	}, event
}

// NewEventLostReadEvent returns the rule and a populated custom event for a lost_events_read event
func NewEventLostReadEvent(mapName string, perCPU map[int]int64, timestamp time.Time) (*rules.Rule, *CustomEvent) {
	return fillCustomRule(&rules.RuleDefinition{
		ID: LostEventsRuleID,
	}, &CustomEvent{
		eventType: "lost_events_read",
		marshalFunc: func() ([]byte, error) {
			return json.Marshal(struct {
				Timestamp time.Time     `json:"timestamp"`
				Name      string        `json:"map"`
				Lost      map[int]int64 `json:"per_cpu"`
			}{
				Timestamp: timestamp,
				Name:      mapName,
				Lost:      perCPU,
			})
		},
	})
}

// NewEventLostWriteEvent returns the rule and a populated custom event for a lost_events_write event
func NewEventLostWriteEvent(mapName string, perEventPerCPU map[string]map[int]uint64, timestamp time.Time) (*rules.Rule, *CustomEvent) {
	return fillCustomRule(&rules.RuleDefinition{
		ID: LostEventsRuleID,
	}, &CustomEvent{
		eventType: "lost_events_write",
		marshalFunc: func() ([]byte, error) {
			return json.Marshal(struct {
				Timestamp time.Time                 `json:"timestamp"`
				Name      string                    `json:"map"`
				Lost      map[string]map[int]uint64 `json:"per_event_per_cpu"`
			}{
				Timestamp: timestamp,
				Name:      mapName,
				Lost:      perEventPerCPU,
			})
		},
	})
}

// NewRuleSetLoadedEvent returns the rule and a populated custom event for a new_rules_loaded event
func NewRuleSetLoadedEvent(loadedPolicies map[string]string, loadedRules []rules.RuleID, loadedMacros []rules.MacroID, timestamp time.Time) (*rules.Rule, *CustomEvent) {
	return fillCustomRule(&rules.RuleDefinition{
		ID: RuleSetLoadedRuleID,
	}, &CustomEvent{
		eventType: RuleSetLoadedRuleID,
		marshalFunc: func() ([]byte, error) {
			return json.Marshal(struct {
				Timestamp time.Time         `json:"timestamp"`
				Policies  map[string]string `json:"policies"`
				Rules     []rules.RuleID    `json:"rules"`
				Macros    []rules.MacroID   `json:"macros"`
			}{
				Timestamp: timestamp,
				Policies:  loadedPolicies,
				Rules:     loadedRules,
				Macros:    loadedMacros,
			})
		},
	})
}

// NewNoisyProcessEvent returns the rule and a populated custom event for a noisy_process event
func NewNoisyProcessEvent(eventType EventType, count uint64, threshold int64, controlPeriod time.Duration, discardedUntil time.Time, process *ProcessCacheEntry, timestamp time.Time) (*rules.Rule, *CustomEvent) {
	return fillCustomRule(&rules.RuleDefinition{
		ID: NoisyProcessRuleID,
	}, &CustomEvent{
		eventType: NoisyProcessRuleID,
		marshalFunc: func() ([]byte, error) {
			return json.Marshal(struct {
				Timestamp      time.Time          `json:"timestamp"`
				Event          string             `json:"event_type"`
				Count          uint64             `json:"pid_count"`
				Threshold      int64              `json:"threshold"`
				ControlPeriod  time.Duration      `json:"control_period"`
				DiscardedUntil time.Time          `json:"discarded_until"`
				Process        *ProcessCacheEntry `json:"process"`
			}{
				Timestamp:      timestamp,
				Event:          eventType.String(),
				Count:          count,
				Threshold:      threshold,
				ControlPeriod:  controlPeriod,
				DiscardedUntil: discardedUntil,
				Process:        process,
			})
		},
	})
}

// NewAbnormalPathEvent returns the rule and a populated custom event for a abnormal_path event
func NewAbnormalPathEvent(event *Event, now time.Time, pathResolutionError error) (*rules.Rule, *CustomEvent) {
	return fillCustomRule(&rules.RuleDefinition{
		ID: AbnormalPathRuleID,
	}, &CustomEvent{
		eventType: event.GetPathResolutionError().Error(),
		marshalFunc: func() ([]byte, error) {
			return json.Marshal(struct {
				Timestamp           time.Time `json:"timestamp"`
				Event               *Event    `json:"triggering_event"`
				PathResolutionError string    `json:"path_resolution_error"`
			}{
				Timestamp:           now,
				Event:               event,
				PathResolutionError: pathResolutionError.Error(),
			})
		},
	})
}

// NewForkBombEvent returns the rule and a populated custom event for a fork_bomb event
func NewForkBombEvent(event *Event, now time.Time) (*rules.Rule, *CustomEvent) {
	return fillCustomRule(&rules.RuleDefinition{
		ID: ForkBombRuleID,
	}, &CustomEvent{
		eventType: ForkBombRuleID,
		marshalFunc: func() ([]byte, error) {
			return json.Marshal(struct {
				Timestamp time.Time `json:"timestamp"`
				Event     *Event    `json:"triggering_event"`
			}{
				Timestamp: now,
				Event:     event,
			})
		},
	})
}
