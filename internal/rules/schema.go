package rules

import (
	"fmt"
	"regexp"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// fileYAML mirrors rules/v1.yaml. It is decoded with unknown fields
// rejected, then validated and converted into Rules.
type fileYAML struct {
	Version     string              `yaml:"version"`
	GameBuild   string              `yaml:"game_build"`
	Units       map[string]unitYAML `yaml:"units"`
	Techs       map[string]techYAML `yaml:"techs"`
	TechCost    techCostYAML        `yaml:"tech_cost"`
	Veterancy   veterancyYAML       `yaml:"veterancy"`
	City        cityYAML            `yaml:"city"`
	UnusedInMVP unusedYAML          `yaml:"unused_in_mvp"`
	Combat      combatYAML          `yaml:"combat"`
}

type unitYAML struct {
	Cost    int      `yaml:"cost"`
	HP      decimal  `yaml:"hp"`
	Attack  decimal  `yaml:"attack"`
	Defense decimal  `yaml:"defense"`
	Move    int      `yaml:"move"`
	Range   int      `yaml:"range"`
	Tech    *string  `yaml:"tech"` // null or absent: no tech needed
	Skills  []string `yaml:"skills"`
}

type techYAML struct {
	Tier     int      `yaml:"tier"`
	Requires *string  `yaml:"requires"` // null or absent: a root tech
	Unlocks  []string `yaml:"unlocks"`
}

type techCostYAML struct {
	Base           *int `yaml:"base"`
	PerTierPerCity int  `yaml:"per_tier_per_city"`
}

type veterancyYAML struct {
	KillsRequired int     `yaml:"kills_required"`
	HPBonus       decimal `yaml:"hp_bonus"`
}

type cityYAML struct {
	LevelUpRewards     map[int][]string `yaml:"level_up_rewards"`
	MVPExcludedRewards []string         `yaml:"mvp_excluded_rewards"`
	MaxCityLevel       int              `yaml:"max_city_level"`
}

type unusedYAML struct {
	Buildings []string `yaml:"buildings"`
	Resources []string `yaml:"resources"`
}

type combatYAML struct {
	Formula      string  `yaml:"formula"`
	DamageScale  decimal `yaml:"damage_scale"`
	DefenseBonus decimal `yaml:"defense_bonus"`
	WallBonus    decimal `yaml:"wall_bonus"`
}

// decimal is a YAML number kept as its source text, so it can be converted
// to tenths exactly. Going through float64 would accept 4.55 and round it.
type decimal struct {
	text string
	set  bool
}

// UnmarshalYAML records a scalar's text. A null leaves the value unset.
func (d *decimal) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: expected a number", n.Line)
	}
	if n.Tag == "!!null" {
		return nil
	}
	d.text, d.set = n.Value, true
	return nil
}

var tenthsPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})(?:\.([0-9]))?$`)

// x10 returns the value in tenths: "4.5" -> 45, "10" -> 100. It accepts only
// non-negative numbers with at most one decimal place.
func (d decimal) x10() (int, bool) {
	m := tenthsPattern.FindStringSubmatch(d.text)
	if m == nil {
		return 0, false
	}
	whole, _ := strconv.Atoi(m[1]) // at most 9 digits: cannot fail or overflow
	tenths := 0
	if m[2] != "" {
		tenths = int(m[2][0] - '0')
	}
	return whole*10 + tenths, true
}
