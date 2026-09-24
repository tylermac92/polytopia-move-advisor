package state

import (
	"reflect"
	"testing"
)

// fixture returns a small state in which every slice is non-empty and every
// slice element is non-zero, so zeroing an element in a clone is a visible
// change.
func fixture() *GameState {
	return &GameState{
		RulesVersion: "v1",
		Turn:         3,
		Active:       1,
		W:            2,
		H:            1,
		Tiles: []Tile{
			{Terrain: Forest, Resource: Game, Owner: 0, CityIdx: 0, Explored: 0b01},
			{Terrain: Mountain, Resource: Metal, Building: Mine, Owner: 1, CityIdx: -1, Road: true, Explored: 0b11},
		},
		Units: []Unit{
			{ID: 1, Kind: 1, Owner: 0, Pos: 0, HP: 100, Kills: 2, HomeCity: 0},
			{ID: 1, Kind: 2, Owner: 1, Pos: 1, HP: 70, Vet: true, Moved: true, HomeCity: 1},
		},
		Cities: []City{
			{Pos: 0, Owner: 0, Level: 2, Population: 1, Capital: true, BorderRadius: 1, PendingReward: 2, UnitCount: 1},
			{Pos: 1, Owner: 1, Level: 3, Population: 4, Walls: true, BorderRadius: 1, UnitCount: 1},
		},
		Players: []Player{
			{ID: 0, Tribe: Imperius, Stars: 5, Techs: TechSet(0).With(0).With(3), Alive: true, NextUnitID: 2},
			{ID: 1, Tribe: Imperius, Stars: 7, Techs: TechSet(0).With(1), Alive: true, NextUnitID: 2},
		},
		RNGSeed: 42,
	}
}

// TestCloneCopiesEverySlice finds GameState's slice fields by reflection, so
// a slice added later is covered without editing this test. For each one it
// zeroes every element in a fresh clone and asserts the parent is unchanged.
func TestCloneCopiesEverySlice(t *testing.T) {
	want := fixture()
	typ := reflect.TypeFor[GameState]()
	slices := 0
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.Slice {
			continue
		}
		slices++
		t.Run(f.Name, func(t *testing.T) {
			parent := fixture()
			clone := parent.Clone()
			if !reflect.DeepEqual(clone, parent) {
				t.Fatalf("clone differs from parent before mutation")
			}
			s := reflect.ValueOf(clone).Elem().Field(i)
			if s.Len() == 0 {
				t.Fatalf("fixture leaves %s empty; mutation would prove nothing", f.Name)
			}
			zero := reflect.Zero(f.Type.Elem())
			for j := 0; j < s.Len(); j++ {
				if s.Index(j).IsZero() {
					t.Fatalf("fixture %s[%d] is zero; mutation would prove nothing", f.Name, j)
				}
				s.Index(j).Set(zero)
			}
			if !reflect.DeepEqual(parent, want) {
				t.Errorf("zeroing clone.%s changed the parent: got %+v", f.Name, reflect.ValueOf(parent).Elem().Field(i))
			}
		})
	}
	if slices != 4 {
		t.Errorf("GameState has %d slice fields, want 4 (Tiles, Units, Cities, Players); update Clone and this check", slices)
	}
}

// TestCloneMutations spells out realistic per-slice edits on a clone, as the
// engine would make them, and checks none reach the parent.
func TestCloneMutations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(c *GameState)
	}{
		{"Tiles", func(c *GameState) { c.Tiles[1].Building = BuildingNone; c.Tiles[0].Owner = 1 }},
		{"Units", func(c *GameState) { c.Units[0].HP = 10; c.Units[1].Moved = false }},
		{"Units removed", func(c *GameState) { c.Units = append(c.Units[:0], c.Units[1:]...) }},
		{"Cities", func(c *GameState) { c.Cities[0].Owner = 1; c.Cities[1].PendingReward = 4 }},
		{"Players", func(c *GameState) {
			c.Players[0].Stars = 0
			c.Players[1].Techs = c.Players[1].Techs.With(9)
			c.Players[0].AllocUnitID()
		}},
		{"scalars", func(c *GameState) { c.Turn++; c.Active = 0; c.RNGSeed = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := fixture()
			tc.mutate(parent.Clone())
			if !reflect.DeepEqual(parent, fixture()) {
				t.Errorf("mutating the clone changed the parent: %+v", parent)
			}
		})
	}
}

func TestCloneNilSlices(t *testing.T) {
	c := (&GameState{}).Clone()
	if c.Tiles != nil || c.Units != nil || c.Cities != nil || c.Players != nil {
		t.Errorf("clone of empty state has non-nil slices: %+v", c)
	}
}

// TestGameStateIsValueTyped enforces the layout Clone relies on: GameState
// has no maps or pointers anywhere, and slices appear only as its own fields,
// never nested inside an element, so one level of copying is a deep copy.
func TestGameStateIsValueTyped(t *testing.T) {
	typ := reflect.TypeFor[GameState]()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		ft := f.Type
		if ft.Kind() == reflect.Slice {
			ft = ft.Elem()
		}
		checkFlat(t, "GameState."+f.Name, ft)
	}
}

func checkFlat(t *testing.T, path string, typ reflect.Type) {
	t.Helper()
	switch typ.Kind() {
	case reflect.Map, reflect.Pointer, reflect.Slice, reflect.Interface,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		t.Errorf("%s is a %s; GameState must stay flat so Clone is a deep copy", path, typ.Kind())
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			checkFlat(t, path+"."+typ.Field(i).Name, typ.Field(i).Type)
		}
	case reflect.Array:
		checkFlat(t, path+"[]", typ.Elem())
	}
}

func TestTechSetIsValueType(t *testing.T) {
	switch k := reflect.TypeFor[TechSet]().Kind(); k {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Array:
	default:
		t.Errorf("TechSet is a %s; it must be an integer or fixed array", k)
	}
}
