package report

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ParamSchema is a simple JSON Schema object definition — a map of property
// names to their type declarations. No $ref, no allOf/anyOf/oneOf gymnastics.
// This is deliberately modest: the schema is what the API exposes to the
// builder UI, and ValidateParams enforces it server-side.
type ParamSchema map[string]ParamProperty

// ParamProperty describes one typed property in a block's parameter schema.
type ParamProperty struct {
	// Type is the JSON type: "string", "number", "integer", "boolean",
	// "array".
	Type string `json:"type"`

	// Description is a human-readable label for the builder UI.
	Description string `json:"description,omitempty"`

	// Default is applied when the property is absent from params.
	Default json.RawMessage `json:"default,omitempty"`

	// Enum restricts string values to a closed set.
	Enum []string `json:"enum,omitempty"`
}

// ValidateParams checks params against schema:
//   - Unknown keys are rejected.
//   - Wrong types are rejected.
//   - Omitted keys receive their default, when one is declared.
//   - A nil or empty schema accepts an empty params object.
//
// It returns the validated params with defaults applied, or an error naming
// the first problem found.
func ValidateParams(schema ParamSchema, params json.RawMessage) (json.RawMessage, error) {
	if schema == nil {
		schema = ParamSchema{}
	}

	var input map[string]json.RawMessage
	if len(params) == 0 || string(params) == "null" {
		input = map[string]json.RawMessage{}
	} else {
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("report: invalid params JSON: %w", err)
		}
	}

	// Reject unknown keys.
	for key := range input {
		if _, ok := schema[key]; !ok {
			return nil, fmt.Errorf("report: unknown param %q", key)
		}
	}

	out := make(map[string]json.RawMessage, len(schema))

	for name, prop := range schema {
		raw, exists := input[name]
		if !exists {
			if prop.Default != nil {
				out[name] = prop.Default
			}
			continue
		}
		if err := checkType(name, prop.Type, raw, prop.Enum); err != nil {
			return nil, err
		}
		out[name] = raw
	}

	result, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("report: marshal validated params: %w", err)
	}
	return json.RawMessage(result), nil
}

func checkType(name, typ string, raw json.RawMessage, enum []string) error {
	switch typ {
	case "string":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fmt.Errorf("report: param %q must be a string", name)
		}
		if len(enum) > 0 {
			for _, allowed := range enum {
				if s == allowed {
					return nil
				}
			}
			return fmt.Errorf("report: param %q must be one of [%s], got %q", name, strings.Join(enum, ", "), s)
		}
	case "number":
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			return fmt.Errorf("report: param %q must be a number", name)
		}
	case "integer":
		// Declared by evidence_appendix's "limit", and unhandled until the
		// block's own default came back through this function on the second
		// save of any report containing it: applyDefaults writes limit: 50
		// into the stored params, the builder echoes what it read, and an
		// unsupported type is an error rather than a pass — so the report
		// became permanently unsaveable.
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			return fmt.Errorf("report: param %q must be an integer", name)
		}
		if n != math.Trunc(n) {
			return fmt.Errorf("report: param %q must be a whole number, got %v", name, n)
		}
	case "boolean":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return fmt.Errorf("report: param %q must be a boolean", name)
		}
	case "array":
		var a []json.RawMessage
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("report: param %q must be an array", name)
		}
	default:
		return fmt.Errorf("report: unsupported param type %q for param %q", typ, name)
	}
	return nil
}
