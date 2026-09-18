package domain

import (
	"fmt"
	"regexp"
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var operationIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func ValidateIdentifier(kind, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%w: %s %q must be kebab-case", ErrInvalidIdentifier, kind, value)
	}
	return nil
}

func ValidateActor(actor Actor) error {
	switch actor.Kind {
	case ActorHuman, ActorAgent, ActorCLI, ActorSystem:
	default:
		return fmt.Errorf("%w: unknown actor kind %q", ErrInvalidActor, actor.Kind)
	}
	if actor.ID == "" {
		return fmt.Errorf("%w: actor id cannot be empty", ErrInvalidActor)
	}
	return nil
}

func ValidateOperationID(value string) error {
	if value == "" {
		return nil
	}
	if !operationIDPattern.MatchString(value) {
		return fmt.Errorf("%w: operation id must use 1-128 letters, numbers, dots, colons, underscores, or hyphens", ErrInvalidIdentifier)
	}
	return nil
}
