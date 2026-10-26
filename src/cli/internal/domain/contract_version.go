package domain

import "fmt"

// ContractSchemaVersion is the only SDD contract version this engine writes and accepts for
// config, capability registry, workflows, work-item manifests (schema_version and
// workflow.version), artifact metadata and archive/cancel transaction markers.
//
// Temporary exception: .sdd/schemas/event.schema.json also accepts "0.1" so historical
// events.jsonl lines stay immutable. The engine never writes "0.1" events (see NewEvent).
// Remove the exception once a migration command/script between versions exists.
const ContractSchemaVersion = "1.0"

// ValidateContractSchemaVersion rejects any document written for another contract version.
//
// @param subject human-readable name of the document being checked (used in the message).
// @param version the schema_version found in the document.
// @returns nil when version equals ContractSchemaVersion, otherwise an error wrapping
// ErrSchemaValidation that names the subject and both versions.
func ValidateContractSchemaVersion(subject, version string) error {
	if version == ContractSchemaVersion {
		return nil
	}
	return fmt.Errorf(
		"%w: %s schema_version %q is not supported; this sdd-cli requires %q (migrate the project manually, see docs/GUIA_INSTALACION.md)",
		ErrSchemaValidation, subject, version, ContractSchemaVersion,
	)
}
