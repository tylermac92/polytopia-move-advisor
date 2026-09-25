package state

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ActionKind says which action an Action is and which of its fields are set.
type ActionKind uint8

// Action kinds. The zero ActionKind is invalid, so a zero Action is never a
// legal action.
const (
	NoAction ActionKind = iota
	Move
	Attack
	Train
	Research
	Harvest
	Build
	CaptureCity
	CityUpgrade
	EndTurn
)

// CityReward is a level-up reward option. Which options each level offers is
// set in the rules file.
type CityReward uint8

// CityReward values. Level 5+ rewards (park, super unit) are out of MVP scope.
const (
	RewardNone CityReward = iota
	RewardWorkshop
	RewardExplorer // excluded from MVP play (random movement); still encodable
	RewardCityWall
	RewardResources
	RewardPopulationGrowth
	RewardBorderGrowth
)

// Action is one engine action: a tagged struct whose Kind says which fields
// are meaningful. Every other field must be zero. The struct has fixed,
// comparable fields, so actions can be compared with == and used as map keys.
//
// Actions refer to units by UnitRef and to cities and targets by tile, never
// by slice index: dead units are removed from GameState.Units mid-plan, so
// unit indexes shift, and a city is always identified by its tile.
//
// Fields used by each kind:
//
//	Move        Unit, Tile (destination)
//	Attack      Unit, Tile (tile of the target unit or city)
//	Train       Tile (city tile), UnitKind
//	Research    Tech
//	Harvest     Tile (resource tile)
//	Build       Tile, Building
//	CaptureCity Unit (standing on the city)
//	CityUpgrade Tile (city tile), Reward
//	EndTurn     none
type Action struct {
	Kind     ActionKind
	Unit     UnitRef
	Tile     TileIdx
	UnitKind UnitKind
	Tech     TechID
	Building Building
	Reward   CityReward
}

// actionField is a bit per optional Action field, used to check that exactly
// the fields a kind uses are set.
type actionField uint8

const (
	fUnit actionField = 1 << iota
	fTile
	fUnitKind
	fTech
	fBuilding
	fReward
)

var actionFields = [...]actionField{
	Move:        fUnit | fTile,
	Attack:      fUnit | fTile,
	Train:       fTile | fUnitKind,
	Research:    fTech,
	Harvest:     fTile,
	Build:       fTile | fBuilding,
	CaptureCity: fUnit,
	CityUpgrade: fTile | fReward,
	EndTurn:     0,
}

// ErrInvalidAction is wrapped by errors for actions whose fields don't match
// their Kind.
var ErrInvalidAction = errors.New("state: invalid action")

// Validate reports whether a is well formed: a known Kind and non-zero values
// only in the fields that kind uses. It does not check legality in any game.
func (a Action) Validate() error {
	if a.Kind == NoAction || int(a.Kind) >= len(actionFields) {
		return fmt.Errorf("%w: kind %v", ErrInvalidAction, a.Kind)
	}
	uses := actionFields[a.Kind]
	check := func(f actionField, name string, isZero bool) error {
		if uses&f == 0 && !isZero {
			return fmt.Errorf("%w: %v sets %s, which it does not use", ErrInvalidAction, a.Kind, name)
		}
		return nil
	}
	return errors.Join(
		check(fUnit, "unit", a.Unit == UnitRef{}),
		check(fTile, "tile", a.Tile == 0),
		check(fUnitKind, "unit_kind", a.UnitKind == 0),
		check(fTech, "tech", a.Tech == 0),
		check(fBuilding, "building", a.Building == BuildingNone),
		check(fReward, "reward", a.Reward == RewardNone),
		a.checkRequired(uses),
	)
}

// checkRequired rejects used fields whose zero value is never meaningful
// (tile 0, unit kind 0 and tech 0 are valid indexes, so those may be zero)
// and techs no TechSet can hold.
func (a Action) checkRequired(uses actionField) error {
	switch {
	case uses&fUnit != 0 && a.Unit.ID == NoUnit:
		return fmt.Errorf("%w: %v needs a unit", ErrInvalidAction, a.Kind)
	case uses&fBuilding != 0 && a.Building == BuildingNone:
		return fmt.Errorf("%w: %v needs a building", ErrInvalidAction, a.Kind)
	case uses&fReward != 0 && a.Reward == RewardNone:
		return fmt.Errorf("%w: %v needs a reward", ErrInvalidAction, a.Kind)
	case a.Tech >= MaxTechs:
		return fmt.Errorf("%w: tech %d out of range (max %d)", ErrInvalidAction, a.Tech, MaxTechs-1)
	}
	return nil
}

// actionJSON is Action's wire form. Pointers mark which fields are present,
// so only the fields the kind uses are written, and a used field whose value
// is zero (tile 0, say) is still distinguishable from an absent one.
type actionJSON struct {
	Kind     ActionKind  `json:"kind"`
	Unit     *UnitRef    `json:"unit,omitempty"`
	Tile     *TileIdx    `json:"tile,omitempty"`
	UnitKind *UnitKind   `json:"unit_kind,omitempty"`
	Tech     *TechID     `json:"tech,omitempty"`
	Building *Building   `json:"building,omitempty"`
	Reward   *CityReward `json:"reward,omitempty"`
}

// MarshalJSON writes the kind and exactly the fields it uses, for example
// {"kind":"move","unit":{"owner":0,"id":3},"tile":12}. It fails if a is not
// valid, so an encoded action always decodes back to an equal one.
func (a Action) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	uses := actionFields[a.Kind]
	w := actionJSON{Kind: a.Kind}
	if uses&fUnit != 0 {
		w.Unit = &a.Unit
	}
	if uses&fTile != 0 {
		w.Tile = &a.Tile
	}
	if uses&fUnitKind != 0 {
		w.UnitKind = &a.UnitKind
	}
	if uses&fTech != 0 {
		w.Tech = &a.Tech
	}
	if uses&fBuilding != 0 {
		w.Building = &a.Building
	}
	if uses&fReward != 0 {
		w.Reward = &a.Reward
	}
	return json.Marshal(w)
}

// UnmarshalJSON reads the form MarshalJSON writes. Every field the kind uses
// must be present, no other field may be, and the result must be valid.
func (a *Action) UnmarshalJSON(data []byte) error {
	var w actionJSON
	if err := decodeStrict(data, &w); err != nil {
		return err
	}
	if w.Kind == NoAction || int(w.Kind) >= len(actionFields) {
		return fmt.Errorf("%w: kind %v", ErrInvalidAction, w.Kind)
	}
	uses := actionFields[w.Kind]
	var present actionField
	out := Action{Kind: w.Kind}
	if w.Unit != nil {
		present |= fUnit
		out.Unit = *w.Unit
	}
	if w.Tile != nil {
		present |= fTile
		out.Tile = *w.Tile
	}
	if w.UnitKind != nil {
		present |= fUnitKind
		out.UnitKind = *w.UnitKind
	}
	if w.Tech != nil {
		present |= fTech
		out.Tech = *w.Tech
	}
	if w.Building != nil {
		present |= fBuilding
		out.Building = *w.Building
	}
	if w.Reward != nil {
		present |= fReward
		out.Reward = *w.Reward
	}
	if present != uses {
		return fmt.Errorf("%w: %v has fields %s, want %s", ErrInvalidAction, w.Kind, present, uses)
	}
	if err := out.Validate(); err != nil {
		return err
	}
	*a = out
	return nil
}

func (f actionField) String() string {
	names := []string{"unit", "tile", "unit_kind", "tech", "building", "reward"}
	s := "["
	for i, n := range names {
		if f&(1<<i) != 0 {
			if s != "[" {
				s += " "
			}
			s += n
		}
	}
	return s + "]"
}
