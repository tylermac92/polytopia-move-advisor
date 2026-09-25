package state

// TileIdx indexes GameState.Tiles; a tile at (x, y) has index y*W+x.
type TileIdx int16

// PlayerID identifies a player and indexes GameState.Players. -1 means no
// player (an unclaimed tile or village).
type PlayerID int8

// NoPlayer is the Owner of unclaimed tiles and cities.
const NoPlayer PlayerID = -1

// UnitID identifies a unit within its owner's units. IDs are assigned from 1
// by the owner's Player.NextUnitID counter; 0 means "no unit".
//
// IDs are per player, not global: with a single global counter the gaps in a
// player's own unit IDs would reveal how many units the enemy trained in
// between. A unit is therefore identified by its UnitRef (owner and ID), never
// by ID alone.
type UnitID int32

// NoUnit is the zero UnitID, which no unit ever has.
const NoUnit UnitID = 0

// UnitRef identifies a unit across the whole game. Two players can own units
// with the same UnitID, so actions and events refer to units by UnitRef.
type UnitRef struct {
	Owner PlayerID `json:"owner"`
	ID    UnitID   `json:"id"`
}

// Terrain is a tile's base terrain.
type Terrain uint8

// Terrain values.
const (
	Field Terrain = iota
	Forest
	Mountain
	Water
	Ocean
)

// Resource is the resource on a tile, if any.
type Resource uint8

// Resource values.
const (
	ResourceNone Resource = iota
	Fruit
	Game
	Crop
	Fish // unused in MVP (naval is out of scope)
	Metal
	Ruin
)

// Building is the building on a tile, if any.
type Building uint8

// Building values.
const (
	BuildingNone Building = iota
	Farm
	LumberHut
	Mine
	Port // unused in MVP (naval is out of scope)
)

// UnitKind indexes the unit types defined in the rules file.
type UnitKind uint8

// Tribe is a player's tribe.
type Tribe uint8

// Tribe values. The MVP covers Imperius only.
const (
	Imperius Tribe = iota
)

// GameState is the complete state of a game. It is flat and value-typed so
// Clone is cheap: it holds no maps or pointers, and every slice's element type
// is itself free of slices, maps and pointers, so copying each slice copies
// the whole state. Units and cities refer to tiles and cities by index.
type GameState struct {
	RulesVersion string   `json:"rules_version"`
	Turn         int      `json:"turn"`
	Active       PlayerID `json:"active"`
	W            int      `json:"w"`
	H            int      `json:"h"`
	Tiles        []Tile   `json:"tiles"`
	Units        []Unit   `json:"units"`  // dense; dead units removed at end of action, so refer to units by UnitRef
	Cities       []City   `json:"cities"` // never removed (capture changes Owner), so indexes are stable
	Players      []Player `json:"players"`
	// RNGSeed is sandbox only; the engine itself is deterministic. It is
	// encoded as a JSON string because JavaScript numbers lose precision
	// above 2^53.
	RNGSeed uint64 `json:"rng_seed,string"`
}

// Tile is one map tile.
type Tile struct {
	Terrain  Terrain  `json:"terrain"`
	Resource Resource `json:"resource"`
	Building Building `json:"building"`
	Owner    PlayerID `json:"owner"`    // NoPlayer if unclaimed
	CityIdx  int16    `json:"city_idx"` // index into Cities; -1 if none
	Road     bool     `json:"road"`
	Explored uint8    `json:"explored"` // bitmask per player: bit p set when player p has explored the tile
}

// Unit is one unit on the map.
type Unit struct {
	ID       UnitID   `json:"id"` // unique only together with Owner; see UnitRef
	Kind     UnitKind `json:"kind"`
	Owner    PlayerID `json:"owner"`
	Pos      TileIdx  `json:"pos"`
	HP       int16    `json:"hp"`    // stored x10 to keep combat in integers
	Kills    int8     `json:"kills"` // counts toward veterancy; threshold in rules file
	Vet      bool     `json:"vet"`
	Moved    bool     `json:"moved"`
	Attacked bool     `json:"attacked"`
	HomeCity int16    `json:"home_city"` // index into Cities; stable because cities are never removed
}

// Ref returns the unit's game-wide identity.
func (u *Unit) Ref() UnitRef { return UnitRef{Owner: u.Owner, ID: u.ID} }

// City is a city or unclaimed village.
//
// Leveling happens one level at a time. When Population crosses the next
// level's threshold, the engine sets PendingReward to the new level and emits
// CityLeveled; while a reward is pending, the only legal actions are that
// level's CityUpgrade options. Choosing the reward clears PendingReward and
// re-checks the threshold, so a population gain spanning two levels produces a
// second CityLeveled rather than two pending rewards.
//
// Level never exceeds the rules file's city.max_city_level (4 in MVP). Growth
// past that cap still adds to Population but never triggers a level-up or
// sets PendingReward, so the engine never reaches a level whose rewards are
// undefined (level 5+ rewards are out of MVP scope).
type City struct {
	Pos          TileIdx  `json:"pos"`
	Owner        PlayerID `json:"owner"`      // NoPlayer for an unclaimed village
	Level        int8     `json:"level"`      // 1..max_city_level from the rules file
	Population   int8     `json:"population"` // keeps growing at max_city_level, without further level-ups
	Capital      bool     `json:"capital"`
	Walls        bool     `json:"walls"`         // level-up reward
	Workshop     bool     `json:"workshop"`      // level-up reward
	BorderRadius int8     `json:"border_radius"` // grows with the border-growth reward
	// PendingReward is the level whose reward has not been chosen yet, or 0
	// if none. At most one reward is pending at a time, and it is never above
	// max_city_level.
	PendingReward int8 `json:"pending_reward"`
	UnitCount     int8 `json:"unit_count"`
}

// Player is one player's economy and research.
type Player struct {
	ID    PlayerID `json:"id"`
	Tribe Tribe    `json:"tribe"`
	Stars int      `json:"stars"`
	Techs TechSet  `json:"techs"` // bitset; must stay a value type (integer or fixed array), never a slice
	Alive bool     `json:"alive"`
	// NextUnitID is the ID this player's next trained unit gets. It starts at
	// 1 and is independent of other players' counters; see UnitID.
	NextUnitID UnitID `json:"next_unit_id"`
}

// AllocUnitID returns a fresh ID for a unit owned by p and advances p's
// counter. A zero counter (a Player built without one) starts at 1.
func (p *Player) AllocUnitID() UnitID {
	if p.NextUnitID < 1 {
		p.NextUnitID = 1
	}
	id := p.NextUnitID
	p.NextUnitID++
	return id
}

// UnitIndex returns the index in s.Units of the unit identified by ref, or -1
// if no such unit is alive. The index is valid only until units are removed.
func (s *GameState) UnitIndex(ref UnitRef) int {
	for i := range s.Units {
		if s.Units[i].Owner == ref.Owner && s.Units[i].ID == ref.ID {
			return i
		}
	}
	return -1
}
