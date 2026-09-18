package domain

import "time"

type ActorKind string

const (
	ActorHuman  ActorKind = "human"
	ActorAgent  ActorKind = "agent"
	ActorCLI    ActorKind = "cli"
	ActorSystem ActorKind = "system"
)

type Actor struct {
	Kind ActorKind `json:"kind" yaml:"kind"`
	ID   string    `json:"id" yaml:"id"`
}

type Event struct {
	SchemaVersion string                 `json:"schema_version" yaml:"schema_version"`
	ID            string                 `json:"id" yaml:"id"`
	At            string                 `json:"at" yaml:"at"`
	WorkItem      string                 `json:"work_item" yaml:"work_item"`
	Type          string                 `json:"type" yaml:"type"`
	Actor         Actor                  `json:"actor" yaml:"actor"`
	Data          map[string]interface{} `json:"data" yaml:"data"`
	CorrelationID string                 `json:"correlation_id,omitempty" yaml:"correlation_id,omitempty"`
}

func NewEvent(id string, at time.Time, workItemID, eventType string, actor Actor, data map[string]interface{}) Event {
	return Event{
		SchemaVersion: "0.1",
		ID:            id,
		At:            at.UTC().Format(time.RFC3339Nano),
		WorkItem:      workItemID,
		Type:          eventType,
		Actor:         actor,
		Data:          data,
	}
}
