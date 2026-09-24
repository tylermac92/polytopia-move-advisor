// Command combat-spike checks the community Polytopia combat formula against
// attacks observed on the pinned build (issue #2). Throwaway: the scenario
// runner in #16 replaces it.
//
// Run: go run ./tools/combat-spike
package main

import (
	"fmt"
	"math"
	"os"
)

// Constants under test. Values match rules/v1.yaml.
const (
	damageScale = 4.5 // community formula multiplier
	cityBonus   = 1.5 // defender in a city without walls
	noBonus     = 1.0
)

type unit struct {
	attack, defense, hp, maxHP float64
}

var warrior = unit{attack: 2, defense: 2, maxHP: 10}

type attack struct {
	name               string
	evidence           string
	attHP, defHP       float64
	defBonus           float64
	obsAttHP, obsDefHP float64
}

// Observed attacks, in chronological order (see testdata/scenarios/shots/).
var observed = []attack{
	{"full HP vs full HP, no bonus", "combat_01", 10, 10, noBonus, 5, 5},
	{"damaged attacker vs full HP, no bonus", "combat_02", 5, 10, noBonus, 0, 7},
	{"damaged attacker vs full HP, city", "combat_03", 5, 10, cityBonus, 0, 8},
	{"full HP vs damaged defender, city", "combat_04", 10, 8, cityBonus, 5, 4},
	{"full HP vs full HP, no bonus (repeat)", "combat_05", 10, 10, noBonus, 5, 5},
}

// roundHalfUp matches the rounding observed in combat_01 (4.5 -> 5).
func roundHalfUp(x float64) float64 { return math.Floor(x + 0.5) }

// resolve applies the community formula. Retaliation is computed from the
// defender's HP before the attack (confirmed by combat_01 and combat_04) and
// only happens if the defender survives.
func resolve(att, def unit, defBonus float64) (attHP, defHP, rawDmg, rawRet float64) {
	attForce := att.attack * att.hp / att.maxHP
	defForce := def.defense * def.hp / def.maxHP * defBonus
	total := attForce + defForce

	rawDmg = attForce / total * att.attack * damageScale
	defHP = math.Max(0, def.hp-roundHalfUp(rawDmg))

	attHP = att.hp
	if defHP > 0 {
		rawRet = defForce / total * def.defense * damageScale
		attHP = math.Max(0, att.hp-roundHalfUp(rawRet))
	}
	return attHP, defHP, rawDmg, rawRet
}

func main() {
	fmt.Printf("%-10s %-40s %9s %9s %11s %11s  %s\n",
		"case", "situation", "raw dmg", "raw ret", "pred A/D", "obs A/D", "result")

	mismatches := 0
	for _, c := range observed {
		a, d := warrior, warrior
		a.hp, d.hp = c.attHP, c.defHP

		pA, pD, rawDmg, rawRet := resolve(a, d, c.defBonus)
		result := "match"
		if pA != c.obsAttHP || pD != c.obsDefHP {
			result = "MISMATCH"
			mismatches++
		}
		fmt.Printf("%-10s %-40s %9.2f %9.2f %5.0f/%-5.0f %5.0f/%-5.0f  %s\n",
			c.evidence, c.name, rawDmg, rawRet, pA, pD, c.obsAttHP, c.obsDefHP, result)
	}

	fmt.Printf("\n%d of %d observed attacks match.\n", len(observed)-mismatches, len(observed))
	if mismatches > 0 {
		os.Exit(1)
	}
}
