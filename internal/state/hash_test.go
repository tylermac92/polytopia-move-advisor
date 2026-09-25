package state

import (
	"fmt"
	"reflect"
	"testing"
)

func TestHashEqualStates(t *testing.T) {
	a, b := fixture(), fixture()
	if a.Hash() != b.Hash() {
		t.Error("identically built states hash differently")
	}
	if a.Hash() != a.Clone().Hash() {
		t.Error("clone hashes differently from its parent")
	}
	if (&GameState{}).Hash() != (&GameState{Tiles: []Tile{}, Units: []Unit{}}).Hash() {
		t.Error("nil and empty slices hash differently; JSON cannot preserve the difference")
	}
}

// TestHashEveryField changes one field at a time, finding fields by
// reflection so a field added later is covered automatically. Every
// single-field change must change the hash, and all the changed hashes must
// also differ from each other, which catches two fields sharing a key.
func TestHashEveryField(t *testing.T) {
	base := fixture().Hash()
	seen := map[uint64]string{base: "unchanged fixture"}
	check := func(path string, s *GameState) {
		t.Helper()
		h := s.Hash()
		if prev, dup := seen[h]; dup {
			t.Errorf("changing %s gives hash %#x, same as %s", path, h, prev)
			return
		}
		seen[h] = path
	}

	typ := reflect.TypeFor[GameState]()
	fields := 0
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.Slice {
			s := fixture()
			bump(t, f.Name, reflect.ValueOf(s).Elem().Field(i))
			check(f.Name, s)
			fields++
			continue
		}
		n := reflect.ValueOf(fixture()).Elem().Field(i).Len()
		for j := 0; j < n; j++ {
			for k := 0; k < f.Type.Elem().NumField(); k++ {
				path := fmt.Sprintf("%s[%d].%s", f.Name, j, f.Type.Elem().Field(k).Name)
				s := fixture()
				bump(t, path, reflect.ValueOf(s).Elem().Field(i).Index(j).Field(k))
				check(path, s)
				fields++
			}
		}
	}
	// The fixture has 72 fields; guard against the walk silently skipping
	// most of them.
	if fields < 50 {
		t.Errorf("only %d fields mutated; reflection walk is broken", fields)
	}
}

// bump changes v to a different value of the same type.
func bump(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(v.Int() + 1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(v.Uint() + 1)
	case reflect.String:
		v.SetString(v.String() + "x")
	default:
		t.Fatalf("%s: don't know how to change a %s; extend bump and Hash", path, v.Kind())
	}
}

func TestHashStructuralChanges(t *testing.T) {
	base := fixture().Hash()
	cases := map[string]func(s *GameState){
		"remove a unit":        func(s *GameState) { s.Units = s.Units[:1] },
		"append a zero tile":   func(s *GameState) { s.Tiles = append(s.Tiles, Tile{}) },
		"append a zero unit":   func(s *GameState) { s.Units = append(s.Units, Unit{}) },
		"append a zero city":   func(s *GameState) { s.Cities = append(s.Cities, City{}) },
		"append a zero player": func(s *GameState) { s.Players = append(s.Players, Player{}) },
		"swap two tiles":       func(s *GameState) { s.Tiles[0], s.Tiles[1] = s.Tiles[1], s.Tiles[0] },
		"swap two cities":      func(s *GameState) { s.Cities[0], s.Cities[1] = s.Cities[1], s.Cities[0] },
		"swap players' techs": func(s *GameState) {
			s.Players[0].Techs, s.Players[1].Techs = s.Players[1].Techs, s.Players[0].Techs
		},
		"swap two units' positions": func(s *GameState) {
			s.Units[0].Pos, s.Units[1].Pos = s.Units[1].Pos, s.Units[0].Pos
		},
		"move a flag between fields": func(s *GameState) {
			s.Units[1].Moved, s.Units[1].Attacked = s.Units[1].Attacked, s.Units[1].Moved
		},
	}
	for name, mutate := range cases {
		s := fixture()
		mutate(s)
		if s.Hash() == base {
			t.Errorf("%s: hash unchanged", name)
		}
	}
}

// Units are identified by UnitRef and the Units slice's order has no game
// meaning, so reordering it must not change the hash.
func TestHashIgnoresUnitOrder(t *testing.T) {
	s := fixture()
	want := s.Hash()
	s.Units[0], s.Units[1] = s.Units[1], s.Units[0]
	if s.Hash() != want {
		t.Error("reordering Units changed the hash")
	}
}

func TestHashJSONRoundTrip(t *testing.T) {
	for name, s := range roundTripStates() {
		data, err := EncodeState(s)
		if err != nil {
			t.Fatalf("%s: EncodeState: %v", name, err)
		}
		got, err := DecodeState(data)
		if err != nil {
			t.Fatalf("%s: DecodeState: %v", name, err)
		}
		if got.Hash() != s.Hash() {
			t.Errorf("%s: hash %#x after JSON round trip, want %#x", name, got.Hash(), s.Hash())
		}
	}
}

// TestHashIsStable pins hash values, so they are the same on every run and
// machine, and a change to the key derivation or field IDs is deliberate.
func TestHashIsStable(t *testing.T) {
	tests := map[string]struct {
		s    *GameState
		want uint64
	}{
		"zero":    {&GameState{}, 0xac20fe2dffa6f172},
		"fixture": {fixture(), 0xc7a81987c1e49076},
	}
	for name, tt := range tests {
		if got := tt.s.Hash(); got != tt.want {
			t.Errorf("%s: Hash() = %#x, want %#x", name, got, tt.want)
		}
	}
}
