package rules

import (
	"math"

	"github.com/tylermac92/polytopia-move-advisor/internal/state"
)

// Rules is a loaded, validated rules file. Every HP value, attack, defense
// and combat constant is an integer in tenths (x10), converted at load time,
// so the engine never does float arithmetic on them.
type Rules struct {
	Version   string
	GameBuild string // platform and version the rules were recorded on

	// Units is indexed by state.UnitKind and Techs by state.TechID. Both are
	// sorted by name, so indexes depend only on the set of names, not on
	// their order in the file.
	Units []Unit
	Techs []Tech

	Veterancy   Veterancy
	City        City
	UnusedInMVP UnusedInMVP
	Combat      Combat

	unitByName map[string]state.UnitKind
	techByName map[string]state.TechID
}

// NoTech marks a unit or tech with no tech requirement.
const NoTech = state.TechID(math.MaxUint8)

// Unit is one unit type's stats.
type Unit struct {
	Name       string
	Cost       int   // stars
	HPx10      int16 // max HP, in the same x10 units as state.Unit.HP
	AttackX10  int
	DefenseX10 int
	Move       int
	Range      int
	Tech       state.TechID // tech needed to train it; NoTech if none
	Skills     SkillSet
}

// Tech is one node of the tech tree.
type Tech struct {
	Name     string
	Tier     int          // 1 for roots; a tech's tier is its parent's plus one
	Requires state.TechID // parent tech; NoTech for tier 1
	Unlocks  []string     // abilities the tech unlocks, as named in the rules file
}

// Veterancy holds the veteran promotion threshold.
type Veterancy struct {
	KillsRequired int
}

// City holds city leveling rules.
type City struct {
	// LevelUpRewards[level] lists the reward options for reaching level, for
	// levels 2..MaxCityLevel, including options excluded from MVP play.
	// Entries below level 2 are nil.
	LevelUpRewards [][]state.CityReward
	// MVPExcludedRewards are never offered in MVP play (explorer moves
	// randomly, and chance nodes are a v2 option).
	MVPExcludedRewards []state.CityReward
	// MaxCityLevel is the MVP level cap. Cities never level past it: growth
	// keeps adding Population but never sets PendingReward.
	MaxCityLevel int
}

// Rewards returns the reward options offered in MVP play on reaching level:
// LevelUpRewards[level] minus MVPExcludedRewards. Validation guarantees at
// least one option for every level from 2 to MaxCityLevel; other levels have
// none.
func (c *City) Rewards(level int) []state.CityReward {
	if level < 2 || level > c.MaxCityLevel {
		return nil
	}
	var out []state.CityReward
	for _, r := range c.LevelUpRewards[level] {
		if !containsReward(c.MVPExcludedRewards, r) {
			out = append(out, r)
		}
	}
	return out
}

// UnusedInMVP lists game content the MVP never generates (naval is out of
// scope). The engine and map generator must not produce these.
type UnusedInMVP struct {
	Buildings []state.Building
	Resources []state.Resource
}

// Combat holds the combat formula and its constants, x10.
type Combat struct {
	Formula         string // only FormulaCommunityV1 is known
	DamageScaleX10  int    // damage multiplier (4.5 -> 45)
	DefenseBonusX10 int    // defense bonus in a city without walls, and terrain bonuses (1.5 -> 15)
	WallBonusX10    int    // defense bonus in a city with walls (4 -> 40)
}

// FormulaCommunityV1 is the community combat formula confirmed in ADR 0001.
const FormulaCommunityV1 = "community_v1"

// UnitKind returns the kind of the unit type called name.
func (r *Rules) UnitKind(name string) (state.UnitKind, bool) {
	k, ok := r.unitByName[name]
	return k, ok
}

// TechID returns the ID of the tech called name.
func (r *Rules) TechID(name string) (state.TechID, bool) {
	t, ok := r.techByName[name]
	return t, ok
}

// Skill is a unit skill from the MVP vocabulary. Skill semantics live in the
// engine; the rules file only says which unit has which.
type Skill uint8

// The MVP skill vocabulary. Semantics: verify on pinned build.
const (
	Dash Skill = iota
	Escape
	Fortify
	Persist
	numSkills
)

var skillNames = [numSkills]string{"dash", "escape", "fortify", "persist"}

func (s Skill) String() string {
	if s < numSkills {
		return skillNames[s]
	}
	return "unknown skill"
}

// SkillSet is a set of skills, as a bitset.
type SkillSet uint8

// Has reports whether s contains k.
func (s SkillSet) Has(k Skill) bool { return s&(1<<k) != 0 }

func containsReward(rs []state.CityReward, r state.CityReward) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
