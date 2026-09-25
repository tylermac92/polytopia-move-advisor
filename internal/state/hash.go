package state

import "hash/fnv"

// Hash returns a Zobrist-style hash of the whole state, for transposition
// detection in search and for checking that two states are the same.
//
// Each hashed item is one (domain, index, field, value) tuple, turned into a
// 64-bit key by zobristKey, and the state's hash is the XOR of all its keys.
// Keys are derived from a fixed seed rather than drawn from a random table,
// so hashes are identical across runs and machines, and fields with large
// ranges (Stars, RNGSeed) need no table. Because the hash is a plain XOR, the
// engine can later update it incrementally by XORing out a field's old key
// and XORing in its new one.
//
// What is hashed:
//   - every GameState scalar field, and the length of each slice;
//   - every field of each tile, city and player, keyed by its slice index;
//   - every field of each unit, keyed by its UnitRef (owner and ID), not its
//     index. Unit order therefore doesn't affect the hash, since units are
//     identified by UnitRef and the dense Units slice's order carries no
//     game meaning.
//
// Two states that differ in any single field always hash differently: for a
// fixed (domain, index, field), zobristKey is a bijection of the value, so
// changing one value changes exactly one key. Unrelated states can still
// collide with probability about 2^-64.
func (s *GameState) Hash() uint64 {
	h := zobristKey(domainGame, 0, gameRulesVersion, fnv64(s.RulesVersion)) ^
		zobristKey(domainGame, 0, gameTurn, int64Bits(int64(s.Turn))) ^
		zobristKey(domainGame, 0, gameActive, int64Bits(int64(s.Active))) ^
		zobristKey(domainGame, 0, gameW, int64Bits(int64(s.W))) ^
		zobristKey(domainGame, 0, gameH, int64Bits(int64(s.H))) ^
		zobristKey(domainGame, 0, gameRNGSeed, s.RNGSeed) ^
		zobristKey(domainGame, 0, gameNumTiles, uint64(len(s.Tiles))) ^
		zobristKey(domainGame, 0, gameNumUnits, uint64(len(s.Units))) ^
		zobristKey(domainGame, 0, gameNumCities, uint64(len(s.Cities))) ^
		zobristKey(domainGame, 0, gameNumPlayers, uint64(len(s.Players)))
	for i := range s.Tiles {
		h ^= s.Tiles[i].hash(uint64(i))
	}
	for i := range s.Units {
		h ^= s.Units[i].hash()
	}
	for i := range s.Cities {
		h ^= s.Cities[i].hash(uint64(i))
	}
	for i := range s.Players {
		h ^= s.Players[i].hash(uint64(i))
	}
	return h
}

func (t *Tile) hash(i uint64) uint64 {
	return zobristKey(domainTile, i, tileTerrain, uint64(t.Terrain)) ^
		zobristKey(domainTile, i, tileResource, uint64(t.Resource)) ^
		zobristKey(domainTile, i, tileBuilding, uint64(t.Building)) ^
		zobristKey(domainTile, i, tileOwner, int64Bits(int64(t.Owner))) ^
		zobristKey(domainTile, i, tileCityIdx, int64Bits(int64(t.CityIdx))) ^
		zobristKey(domainTile, i, tileRoad, boolBits(t.Road)) ^
		zobristKey(domainTile, i, tileExplored, uint64(t.Explored))
}

func (u *Unit) hash() uint64 {
	// Owner and ID are the unit's key; both are folded into the index, so
	// neither needs a field of its own.
	i := uint64(uint8(u.Owner))<<32 | uint64(uint32(u.ID))
	return zobristKey(domainUnit, i, unitKind, uint64(u.Kind)) ^
		zobristKey(domainUnit, i, unitPos, int64Bits(int64(u.Pos))) ^
		zobristKey(domainUnit, i, unitHP, int64Bits(int64(u.HP))) ^
		zobristKey(domainUnit, i, unitKills, int64Bits(int64(u.Kills))) ^
		zobristKey(domainUnit, i, unitVet, boolBits(u.Vet)) ^
		zobristKey(domainUnit, i, unitMoved, boolBits(u.Moved)) ^
		zobristKey(domainUnit, i, unitAttacked, boolBits(u.Attacked)) ^
		zobristKey(domainUnit, i, unitHomeCity, int64Bits(int64(u.HomeCity)))
}

func (c *City) hash(i uint64) uint64 {
	return zobristKey(domainCity, i, cityPos, int64Bits(int64(c.Pos))) ^
		zobristKey(domainCity, i, cityOwner, int64Bits(int64(c.Owner))) ^
		zobristKey(domainCity, i, cityLevel, int64Bits(int64(c.Level))) ^
		zobristKey(domainCity, i, cityPopulation, int64Bits(int64(c.Population))) ^
		zobristKey(domainCity, i, cityCapital, boolBits(c.Capital)) ^
		zobristKey(domainCity, i, cityWalls, boolBits(c.Walls)) ^
		zobristKey(domainCity, i, cityWorkshop, boolBits(c.Workshop)) ^
		zobristKey(domainCity, i, cityBorderRadius, int64Bits(int64(c.BorderRadius))) ^
		zobristKey(domainCity, i, cityPendingReward, int64Bits(int64(c.PendingReward))) ^
		zobristKey(domainCity, i, cityUnitCount, int64Bits(int64(c.UnitCount)))
}

func (p *Player) hash(i uint64) uint64 {
	return zobristKey(domainPlayer, i, playerID, int64Bits(int64(p.ID))) ^
		zobristKey(domainPlayer, i, playerTribe, uint64(p.Tribe)) ^
		zobristKey(domainPlayer, i, playerStars, int64Bits(int64(p.Stars))) ^
		zobristKey(domainPlayer, i, playerTechs, uint64(p.Techs)) ^
		zobristKey(domainPlayer, i, playerAlive, boolBits(p.Alive)) ^
		zobristKey(domainPlayer, i, playerNextUnitID, int64Bits(int64(p.NextUnitID)))
}

// zobristSeed fixes every key. Changing it changes every hash; nothing
// persists hashes today, but TestHashIsStable pins them anyway so a change is
// deliberate.
const zobristSeed uint64 = 0x9e3779b97f4a7c15

// zobristKey derives the key for one hashed value. Each component is XORed
// into the running state and passed through mix64, which is a bijection, so
// for a fixed domain, index and field, distinct values always give distinct
// keys.
func zobristKey(domain, index, field, value uint64) uint64 {
	h := mix64(zobristSeed ^ domain)
	h = mix64(h ^ index)
	h = mix64(h ^ field)
	return mix64(h ^ value)
}

// mix64 is the SplitMix64 finalizer: a bijection on uint64 with good
// avalanche, so nearby inputs give unrelated outputs.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

// int64Bits sign-extends so that negative values (such as -1 sentinels)
// map to distinct keys from every non-negative value.
func int64Bits(v int64) uint64 { return uint64(v) }

func boolBits(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func fnv64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s)) // hash.Hash.Write never returns an error
	return h.Sum64()
}

// Hash domains and field IDs. The values are part of every key, so they are
// fixed numbers rather than iota tied to struct field order: reordering a
// struct must not change hashes.
const (
	domainGame   uint64 = 1
	domainTile   uint64 = 2
	domainUnit   uint64 = 3
	domainCity   uint64 = 4
	domainPlayer uint64 = 5
)

const (
	gameRulesVersion uint64 = 1
	gameTurn         uint64 = 2
	gameActive       uint64 = 3
	gameW            uint64 = 4
	gameH            uint64 = 5
	gameRNGSeed      uint64 = 6
	gameNumTiles     uint64 = 7
	gameNumUnits     uint64 = 8
	gameNumCities    uint64 = 9
	gameNumPlayers   uint64 = 10
)

const (
	tileTerrain  uint64 = 1
	tileResource uint64 = 2
	tileBuilding uint64 = 3
	tileOwner    uint64 = 4
	tileCityIdx  uint64 = 5
	tileRoad     uint64 = 6
	tileExplored uint64 = 7
)

const (
	unitKind     uint64 = 1
	unitPos      uint64 = 2
	unitHP       uint64 = 3
	unitKills    uint64 = 4
	unitVet      uint64 = 5
	unitMoved    uint64 = 6
	unitAttacked uint64 = 7
	unitHomeCity uint64 = 8
)

const (
	cityPos           uint64 = 1
	cityOwner         uint64 = 2
	cityLevel         uint64 = 3
	cityPopulation    uint64 = 4
	cityCapital       uint64 = 5
	cityWalls         uint64 = 6
	cityWorkshop      uint64 = 7
	cityBorderRadius  uint64 = 8
	cityPendingReward uint64 = 9
	cityUnitCount     uint64 = 10
)

const (
	playerID         uint64 = 1
	playerTribe      uint64 = 2
	playerStars      uint64 = 3
	playerTechs      uint64 = 4
	playerAlive      uint64 = 5
	playerNextUnitID uint64 = 6
)
