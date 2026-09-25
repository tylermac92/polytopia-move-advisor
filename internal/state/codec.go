package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// SchemaVersion is the version of the JSON encoding written by EncodeState
// and EncodeAction. Bump it whenever the encoding changes in a way an older
// decoder would misread; decoders accept only this exact version.
const SchemaVersion = 1

// SchemaVersionError is returned when a document's schema version is not
// SchemaVersion. A missing version decodes as Got == 0.
type SchemaVersionError struct {
	Got, Want int
}

func (e *SchemaVersionError) Error() string {
	return fmt.Sprintf("state: schema version %d, want %d", e.Got, e.Want)
}

// stateDoc is the top-level form of an encoded GameState: the schema version
// followed by the state's own fields, in one flat object.
type stateDoc struct {
	Schema int `json:"schema"`
	*GameState
}

// actionDoc is the top-level form of an encoded Action. The action's fields
// are written by Action.MarshalJSON, so they are nested under "action".
type actionDoc struct {
	Schema int    `json:"schema"`
	Action Action `json:"action"`
}

// EncodeState encodes s as a JSON document carrying SchemaVersion.
func EncodeState(s *GameState) ([]byte, error) {
	return json.Marshal(stateDoc{Schema: SchemaVersion, GameState: s})
}

// DecodeState decodes a document written by EncodeState. It returns a
// *SchemaVersionError if the document has a different schema version, and
// rejects unknown fields and trailing data.
func DecodeState(data []byte) (*GameState, error) {
	if err := checkSchema(data); err != nil {
		return nil, err
	}
	doc := stateDoc{GameState: &GameState{}}
	if err := decodeStrict(data, &doc); err != nil {
		return nil, fmt.Errorf("state: decode game state: %w", err)
	}
	return doc.GameState, nil
}

// EncodeAction encodes a as a standalone JSON document carrying
// SchemaVersion, for example
// {"schema":1,"action":{"kind":"end_turn"}}. Actions embedded in a larger
// document use Action's own MarshalJSON, which has no version field.
func EncodeAction(a Action) ([]byte, error) {
	return json.Marshal(actionDoc{Schema: SchemaVersion, Action: a})
}

// DecodeAction decodes a document written by EncodeAction. It returns a
// *SchemaVersionError if the document has a different schema version.
func DecodeAction(data []byte) (Action, error) {
	if err := checkSchema(data); err != nil {
		return Action{}, err
	}
	var doc actionDoc
	if err := decodeStrict(data, &doc); err != nil {
		return Action{}, fmt.Errorf("state: decode action: %w", err)
	}
	return doc.Action, nil
}

// checkSchema reads only the schema field, so a document from another
// version fails with a *SchemaVersionError rather than whatever field error
// its different layout would cause.
func checkSchema(data []byte) error {
	var v struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("state: read schema version: %w", err)
	}
	if v.Schema != SchemaVersion {
		return &SchemaVersionError{Got: v.Schema, Want: SchemaVersion}
	}
	return nil
}

// decodeStrict decodes exactly one JSON value into v, rejecting unknown
// fields and trailing data.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("unexpected data after JSON value")
	}
	return nil
}

// MarshalJSON encodes s as the sorted list of tech IDs it holds, which stays
// readable and exact in JavaScript (a 64-bit integer would not).
func (s TechSet) MarshalJSON() ([]byte, error) {
	// []int, not []TechID: encoding/json writes byte-sized slices as base64.
	ids := make([]int, 0, s.Len())
	for t := TechID(0); t < MaxTechs; t++ {
		if s.Has(t) {
			ids = append(ids, int(t))
		}
	}
	return json.Marshal(ids)
}

// UnmarshalJSON decodes a list of tech IDs. IDs must be below MaxTechs and
// listed at most once.
func (s *TechSet) UnmarshalJSON(data []byte) error {
	var ids []int
	if err := json.Unmarshal(data, &ids); err != nil {
		return err
	}
	var out TechSet
	for _, id := range ids {
		if id < 0 || id >= MaxTechs {
			return fmt.Errorf("state: tech %d out of range (max %d)", id, MaxTechs-1)
		}
		t := TechID(id)
		if out.Has(t) {
			return fmt.Errorf("state: tech %d listed twice", t)
		}
		out = out.With(t)
	}
	*s = out
	return nil
}
