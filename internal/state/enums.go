package state

import "fmt"

// Enums are encoded in JSON by name rather than by number, so saved games and
// scenario files stay readable and don't silently change meaning if constants
// are reordered. UnitKind and TechID are indexes into the rules file, whose
// names the state package doesn't know, so they stay numeric.

var (
	terrainNames    = []string{"field", "forest", "mountain", "water", "ocean"}
	resourceNames   = []string{"none", "fruit", "game", "crop", "fish", "metal", "ruin"}
	buildingNames   = []string{"none", "farm", "lumber_hut", "mine", "port"}
	tribeNames      = []string{"imperius"}
	cityRewardNames = []string{"none", "workshop", "explorer", "city_wall", "resources", "population_growth", "border_growth"}
	actionKindNames = []string{"none", "move", "attack", "train", "research", "harvest", "build", "capture_city", "city_upgrade", "end_turn"}
)

func enumString[T ~uint8](v T, names []string, typ string) string {
	if int(v) < len(names) {
		return names[v]
	}
	return fmt.Sprintf("%s(%d)", typ, v)
}

func marshalEnum[T ~uint8](v T, names []string, typ string) ([]byte, error) {
	if int(v) >= len(names) {
		return nil, fmt.Errorf("state: invalid %s %d", typ, v)
	}
	return []byte(names[v]), nil
}

func unmarshalEnum[T ~uint8](dst *T, text []byte, names []string, typ string) error {
	for i, n := range names {
		if n == string(text) {
			*dst = T(i)
			return nil
		}
	}
	return fmt.Errorf("state: unknown %s %q", typ, text)
}

func (t Terrain) String() string { return enumString(t, terrainNames, "Terrain") }

// MarshalText encodes t by name.
func (t Terrain) MarshalText() ([]byte, error) { return marshalEnum(t, terrainNames, "Terrain") }

// UnmarshalText decodes a name produced by MarshalText.
func (t *Terrain) UnmarshalText(b []byte) error {
	return unmarshalEnum(t, b, terrainNames, "Terrain")
}

func (r Resource) String() string { return enumString(r, resourceNames, "Resource") }

// MarshalText encodes r by name.
func (r Resource) MarshalText() ([]byte, error) { return marshalEnum(r, resourceNames, "Resource") }

// UnmarshalText decodes a name produced by MarshalText.
func (r *Resource) UnmarshalText(b []byte) error {
	return unmarshalEnum(r, b, resourceNames, "Resource")
}

func (b Building) String() string { return enumString(b, buildingNames, "Building") }

// MarshalText encodes b by name.
func (b Building) MarshalText() ([]byte, error) { return marshalEnum(b, buildingNames, "Building") }

// UnmarshalText decodes a name produced by MarshalText.
func (b *Building) UnmarshalText(text []byte) error {
	return unmarshalEnum(b, text, buildingNames, "Building")
}

func (t Tribe) String() string { return enumString(t, tribeNames, "Tribe") }

// MarshalText encodes t by name.
func (t Tribe) MarshalText() ([]byte, error) { return marshalEnum(t, tribeNames, "Tribe") }

// UnmarshalText decodes a name produced by MarshalText.
func (t *Tribe) UnmarshalText(b []byte) error { return unmarshalEnum(t, b, tribeNames, "Tribe") }

func (r CityReward) String() string { return enumString(r, cityRewardNames, "CityReward") }

// MarshalText encodes r by name.
func (r CityReward) MarshalText() ([]byte, error) {
	return marshalEnum(r, cityRewardNames, "CityReward")
}

// UnmarshalText decodes a name produced by MarshalText.
func (r *CityReward) UnmarshalText(b []byte) error {
	return unmarshalEnum(r, b, cityRewardNames, "CityReward")
}

func (k ActionKind) String() string { return enumString(k, actionKindNames, "ActionKind") }

// MarshalText encodes k by name.
func (k ActionKind) MarshalText() ([]byte, error) {
	return marshalEnum(k, actionKindNames, "ActionKind")
}

// UnmarshalText decodes a name produced by MarshalText.
func (k *ActionKind) UnmarshalText(b []byte) error {
	return unmarshalEnum(k, b, actionKindNames, "ActionKind")
}
