package state

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

// roundTripStates are hand-built states covering empty and nil slices,
// every enum value, sentinel values (-1 owners and indexes) and extremes.
func roundTripStates() map[string]*GameState {
	boundary := &GameState{
		RulesVersion: "v1",
		Turn:         math.MaxInt32,
		Active:       0,
		W:            4,
		H:            2,
		RNGSeed:      math.MaxUint64, // above 2^53: must survive as a string
		Players: []Player{
			{ID: 0, Tribe: Imperius, Stars: math.MaxInt32, Techs: ^TechSet(0), Alive: true, NextUnitID: math.MaxInt32},
			{ID: 1, Tribe: Imperius, Stars: 0, Techs: 0, Alive: false, NextUnitID: 1},
		},
		Cities: []City{
			{Pos: 0, Owner: NoPlayer, Level: 1},
			{Pos: 7, Owner: 0, Level: 4, Population: math.MaxInt8, Capital: true, Walls: true, Workshop: true,
				BorderRadius: 2, PendingReward: 4, UnitCount: 3},
		},
		Units: []Unit{
			{ID: 1, Kind: 0, Owner: 0, Pos: 7, HP: 150, Kills: 3, Vet: true, Moved: true, Attacked: true, HomeCity: 1},
			{ID: 1, Kind: math.MaxUint8, Owner: 1, Pos: 6, HP: 1, HomeCity: -1},
		},
	}
	// One tile per terrain, then one per resource and building, so every
	// enum value is encoded at least once.
	for i := range terrainNames {
		boundary.Tiles = append(boundary.Tiles, Tile{Terrain: Terrain(i), Owner: NoPlayer, CityIdx: -1})
	}
	for i := range resourceNames {
		boundary.Tiles = append(boundary.Tiles, Tile{Resource: Resource(i), Owner: 1, CityIdx: -1, Explored: 0b11})
	}
	for i := range buildingNames {
		boundary.Tiles = append(boundary.Tiles, Tile{Building: Building(i), Owner: 0, CityIdx: 1, Road: true, Explored: math.MaxUint8})
	}

	return map[string]*GameState{
		"zero":          {},
		"empty slices":  {Tiles: []Tile{}, Units: []Unit{}, Cities: []City{}, Players: []Player{}},
		"clone fixture": fixture(),
		"boundary":      boundary,
	}
}

func TestStateRoundTrip(t *testing.T) {
	for name, want := range roundTripStates() {
		t.Run(name, func(t *testing.T) {
			data, err := EncodeState(want)
			if err != nil {
				t.Fatalf("EncodeState: %v", err)
			}
			got, err := DecodeState(data)
			if err != nil {
				t.Fatalf("DecodeState: %v\n%s", err, data)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("round trip changed the state\n got %+v\nwant %+v\njson %s", got, want, data)
			}
			again, err := EncodeState(got)
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}
			if string(again) != string(data) {
				t.Errorf("re-encoding is not byte-identical\nfirst  %s\nsecond %s", data, again)
			}
		})
	}
}

// TestStateWireFormat pins the encoding of a small state, so a change to the
// format shows up here and prompts a SchemaVersion bump.
func TestStateWireFormat(t *testing.T) {
	s := &GameState{
		RulesVersion: "v1", Turn: 2, Active: 1, W: 1, H: 1, RNGSeed: 7,
		Tiles:   []Tile{{Terrain: Forest, Resource: Game, Building: LumberHut, Owner: 0, CityIdx: -1, Road: true, Explored: 1}},
		Units:   []Unit{{ID: 3, Kind: 1, Owner: 0, Pos: 0, HP: 100, Kills: 1, HomeCity: 0}},
		Cities:  []City{{Pos: 0, Owner: 0, Level: 2, Population: 1, Capital: true, BorderRadius: 1, PendingReward: 2}},
		Players: []Player{{ID: 0, Tribe: Imperius, Stars: 5, Techs: TechSet(0).With(0).With(4), Alive: true, NextUnitID: 4}},
	}
	want := `{"schema":1,"rules_version":"v1","turn":2,"active":1,"w":1,"h":1,` +
		`"tiles":[{"terrain":"forest","resource":"game","building":"lumber_hut","owner":0,"city_idx":-1,"road":true,"explored":1}],` +
		`"units":[{"id":3,"kind":1,"owner":0,"pos":0,"hp":100,"kills":1,"vet":false,"moved":false,"attacked":false,"home_city":0}],` +
		`"cities":[{"pos":0,"owner":0,"level":2,"population":1,"capital":true,"walls":false,"workshop":false,"border_radius":1,"pending_reward":2,"unit_count":0}],` +
		`"players":[{"id":0,"tribe":"imperius","stars":5,"techs":[0,4],"alive":true,"next_unit_id":4}],` +
		`"rng_seed":"7"}`
	got, err := EncodeState(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("wire format changed\n got %s\nwant %s", got, want)
	}
}

// roundTripActions has one or more actions of every kind, including fields
// whose valid value is zero (tile 0, tech 0, unit kind 0).
func roundTripActions() []Action {
	u := UnitRef{Owner: 1, ID: 7}
	return []Action{
		{Kind: Move, Unit: u, Tile: 12},
		{Kind: Move, Unit: UnitRef{Owner: 0, ID: 1}, Tile: 0},
		{Kind: Attack, Unit: u, Tile: math.MaxInt16},
		{Kind: Train, Tile: 3, UnitKind: 0},
		{Kind: Train, Tile: 3, UnitKind: 2},
		{Kind: Research, Tech: 0},
		{Kind: Research, Tech: MaxTechs - 1},
		{Kind: Harvest, Tile: 5},
		{Kind: Build, Tile: 5, Building: Farm},
		{Kind: CaptureCity, Unit: u},
		{Kind: CityUpgrade, Tile: 9, Reward: RewardCityWall},
		{Kind: CityUpgrade, Tile: 9, Reward: RewardBorderGrowth},
		{Kind: EndTurn},
	}
}

func TestActionRoundTrip(t *testing.T) {
	covered := map[ActionKind]bool{}
	for _, want := range roundTripActions() {
		covered[want.Kind] = true
		data, err := EncodeAction(want)
		if err != nil {
			t.Errorf("EncodeAction(%+v): %v", want, err)
			continue
		}
		got, err := DecodeAction(data)
		if err != nil {
			t.Errorf("DecodeAction(%s): %v", data, err)
			continue
		}
		if got != want {
			t.Errorf("round trip of %s gave %+v, want %+v", data, got, want)
		}
	}
	for k := NoAction + 1; int(k) < len(actionFields); k++ {
		if !covered[k] {
			t.Errorf("no round-trip case for %v", k)
		}
	}
}

func TestActionWireFormat(t *testing.T) {
	tests := []struct {
		a    Action
		want string
	}{
		{Action{Kind: Move, Unit: UnitRef{Owner: 0, ID: 3}, Tile: 12}, `{"schema":1,"action":{"kind":"move","unit":{"owner":0,"id":3},"tile":12}}`},
		{Action{Kind: Research, Tech: 0}, `{"schema":1,"action":{"kind":"research","tech":0}}`},
		{Action{Kind: CityUpgrade, Tile: 4, Reward: RewardWorkshop}, `{"schema":1,"action":{"kind":"city_upgrade","tile":4,"reward":"workshop"}}`},
		{Action{Kind: EndTurn}, `{"schema":1,"action":{"kind":"end_turn"}}`},
	}
	for _, tt := range tests {
		got, err := EncodeAction(tt.a)
		if err != nil {
			t.Errorf("EncodeAction(%+v): %v", tt.a, err)
			continue
		}
		if string(got) != tt.want {
			t.Errorf("EncodeAction(%+v)\n got %s\nwant %s", tt.a, got, tt.want)
		}
	}
}

func TestActionIsComparable(t *testing.T) {
	seen := map[Action]int{}
	for _, a := range roundTripActions() {
		seen[a]++
	}
	if len(seen) != len(roundTripActions()) {
		t.Errorf("distinct actions collided as map keys")
	}
}

func TestEncodeInvalidAction(t *testing.T) {
	bad := []Action{
		{},
		{Kind: ActionKind(len(actionKindNames))},
		{Kind: Move, Tile: 3},    // no unit
		{Kind: EndTurn, Tile: 1}, // tile on a kind that has none
		{Kind: Research, Tech: 1, Unit: UnitRef{ID: 1}}, // unit on research
		{Kind: Research, Tech: MaxTechs},
		{Kind: Build, Tile: 2},                                      // no building
		{Kind: CityUpgrade, Tile: 2},                                // no reward
		{Kind: Train, Tile: 2, UnitKind: 1, Reward: RewardWorkshop}, // stray reward
	}
	for _, a := range bad {
		if _, err := EncodeAction(a); !errors.Is(err, ErrInvalidAction) {
			t.Errorf("EncodeAction(%+v) error = %v, want ErrInvalidAction", a, err)
		}
	}
}

func TestDecodeInvalidAction(t *testing.T) {
	bad := []string{
		`{"schema":1,"action":{"kind":"move","unit":{"owner":0,"id":3}}}`,                // missing tile
		`{"schema":1,"action":{"kind":"end_turn","tile":0}}`,                             // extra tile, even if zero
		`{"schema":1,"action":{"kind":"research","tech":64}}`,                            // no TechSet holds tech 64
		`{"schema":1,"action":{"kind":"fly"}}`,                                           // unknown kind
		`{"schema":1,"action":{"kind":"harvest","tile":1,"colour":2}}`,                   // unknown field
		`{"schema":1,"action":{"kind":"build","tile":1,"building":"none"}}`,              // building required
		`{"schema":1,"action":{"kind":"capture_city","unit":{"owner":0,"id":1,"hp":3}}}`, // unknown nested field
		`{"schema":1,"action":{"kind":"end_turn"}} {}`,                                   // trailing data
	}
	for _, doc := range bad {
		if a, err := DecodeAction([]byte(doc)); err == nil {
			t.Errorf("DecodeAction(%s) = %+v, want error", doc, a)
		}
	}
}

func TestDecodeSchemaMismatch(t *testing.T) {
	docs := map[string]struct {
		doc     string
		gotWant int
	}{
		"newer":   {`{"schema":2,"turn":1}`, 2},
		"older":   {`{"schema":0}`, 0},
		"missing": {`{"turn":1}`, 0},
		// A newer document may have fields this version doesn't know; the
		// version error must win over the unknown-field error.
		"newer with new fields": {`{"schema":2,"weather":"rain","action":{"kind":"swim"}}`, 2},
	}
	for name, tt := range docs {
		t.Run(name, func(t *testing.T) {
			_, errState := DecodeState([]byte(tt.doc))
			_, errAction := DecodeAction([]byte(tt.doc))
			for label, err := range map[string]error{"DecodeState": errState, "DecodeAction": errAction} {
				var sv *SchemaVersionError
				if !errors.As(err, &sv) {
					t.Errorf("%s error = %v, want *SchemaVersionError", label, err)
					continue
				}
				if sv.Got != tt.gotWant || sv.Want != SchemaVersion {
					t.Errorf("%s error = %+v, want Got %d Want %d", label, sv, tt.gotWant, SchemaVersion)
				}
			}
		})
	}
}

func TestDecodeStateRejects(t *testing.T) {
	bad := map[string]string{
		"unknown field":   `{"schema":1,"turn":1,"score":3}`,
		"unknown terrain": `{"schema":1,"tiles":[{"terrain":"lava"}]}`,
		"numeric enum":    `{"schema":1,"tiles":[{"terrain":1}]}`,
		"tech too large":  `{"schema":1,"players":[{"techs":[64]}]}`,
		"tech twice":      `{"schema":1,"players":[{"techs":[1,1]}]}`,
		"numeric seed":    `{"schema":1,"rng_seed":7}`,
		"trailing data":   `{"schema":1}{"schema":1}`,
		"not json":        `schema: 1`,
	}
	for name, doc := range bad {
		if s, err := DecodeState([]byte(doc)); err == nil {
			t.Errorf("%s: DecodeState(%s) = %+v, want error", name, doc, s)
		}
	}
}

func TestEncodeStateRejectsInvalidEnum(t *testing.T) {
	s := &GameState{Tiles: []Tile{{Terrain: Terrain(len(terrainNames))}}}
	_, err := EncodeState(s)
	if err == nil || !strings.Contains(err.Error(), "invalid Terrain") {
		t.Errorf("EncodeState with bad terrain: error = %v", err)
	}
}
