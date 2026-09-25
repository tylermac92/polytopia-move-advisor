package rules

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tylermac92/polytopia-move-advisor/internal/state"
)

// validYAML is a small, complete, valid rules file. Rejection tests edit one
// thing in it, so each failure has a single cause.
const validYAML = `
version: v1
game_build: "Android 2.17.3.16375"
units:
  warrior: { cost: 2, hp: 10, attack: 2, defense: 2, move: 1, range: 1, tech: null, skills: [dash, fortify] }
  archer:  { cost: 3, hp: 10, attack: 2, defense: 1, move: 1, range: 2, tech: archery, skills: [dash, fortify] }
  knight:  { cost: 8, hp: 10, attack: 3.5, defense: 1, move: 3, range: 1, tech: chivalry, skills: [dash, persist, fortify] }
  catapult: { cost: 8, hp: 10, attack: 4, defense: 0, move: 1, range: 3, tech: archery }
techs:
  hunting:      { tier: 1, requires: null, unlocks: [harvest_game] }
  archery:      { tier: 2, requires: hunting, unlocks: [forest_defense] }
  riding:       { tier: 1 }
  free_spirit:  { tier: 2, requires: riding }
  chivalry:     { tier: 3, requires: free_spirit }
tech_cost:
  base: 4
  per_tier_per_city: 1
veterancy:
  kills_required: 3
  hp_bonus: 5
city:
  level_up_rewards:
    2: [workshop, explorer]
    3: [city_wall, resources]
    4: [population_growth, border_growth]
  mvp_excluded_rewards: [explorer]
  max_city_level: 4
unused_in_mvp:
  buildings: [port]
  resources: [fish]
combat:
  formula: community_v1
  damage_scale: 4.5
  defense_bonus: 1.5
  wall_bonus: 4
`

// edit returns validYAML with old replaced by new, failing if old is absent
// so a test can't silently check the unedited file.
func edit(t *testing.T, old, new string) string {
	t.Helper()
	if !strings.Contains(validYAML, old) {
		t.Fatalf("validYAML does not contain %q", old)
	}
	return strings.Replace(validYAML, old, new, 1)
}

// noUnits returns validYAML with an empty units section.
func noUnits(t *testing.T) string {
	t.Helper()
	start := strings.Index(validYAML, "units:\n")
	end := strings.Index(validYAML, "techs:\n")
	if start < 0 || end < start {
		t.Fatal("validYAML layout changed")
	}
	return validYAML[:start] + "units: {}\n" + validYAML[end:]
}

func TestParseValid(t *testing.T) {
	r, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	k, ok := r.UnitKind("knight")
	if !ok {
		t.Fatal("knight not found")
	}
	chivalry, _ := r.TechID("chivalry")
	want := Unit{
		Name: "knight", Cost: 8, HPx10: 100, AttackX10: 35, DefenseX10: 10, Move: 3, Range: 1,
		Tech: chivalry, Skills: 1<<Dash | 1<<Persist | 1<<Fortify,
	}
	if got := r.Units[k]; !reflect.DeepEqual(got, want) {
		t.Errorf("knight = %+v, want %+v", got, want)
	}
	if w, _ := r.UnitKind("warrior"); r.Units[w].Tech != NoTech {
		t.Errorf("warrior tech = %d, want NoTech", r.Units[w].Tech)
	}
	if got := r.Units[k].Skills.Has(Escape); got {
		t.Error("knight has escape")
	}

	wantCombat := Combat{Formula: FormulaCommunityV1, DamageScaleX10: 45, DefenseBonusX10: 15, WallBonusX10: 40}
	if r.Combat != wantCombat {
		t.Errorf("Combat = %+v, want %+v", r.Combat, wantCombat)
	}
	if c, _ := r.UnitKind("catapult"); r.Units[c].DefenseX10 != 0 || r.Units[c].Skills != 0 {
		t.Errorf("catapult = %+v", r.Units[c])
	}
	if r.TechCost != (TechCost{Base: 4, PerTierPerCity: 1}) || r.TechCost.Cost(3, 2) != 10 {
		t.Errorf("TechCost = %+v, Cost(3, 2) = %d", r.TechCost, r.TechCost.Cost(3, 2))
	}
	if r.Veterancy.HPBonusX10 != 50 {
		t.Errorf("HPBonusX10 = %d, want 50", r.Veterancy.HPBonusX10)
	}
	if r.Veterancy.KillsRequired != 3 || r.City.MaxCityLevel != 4 {
		t.Errorf("Veterancy %+v, MaxCityLevel %d", r.Veterancy, r.City.MaxCityLevel)
	}

	// Indexes follow sorted names, whatever the file order.
	var names []string
	for _, tech := range r.Techs {
		names = append(names, tech.Name)
	}
	if want := []string{"archery", "chivalry", "free_spirit", "hunting", "riding"}; !reflect.DeepEqual(names, want) {
		t.Errorf("tech order %v, want %v", names, want)
	}
	archery, _ := r.TechID("archery")
	hunting, _ := r.TechID("hunting")
	if r.Techs[archery].Requires != hunting || r.Techs[hunting].Requires != NoTech {
		t.Errorf("requires links wrong: %+v", r.Techs)
	}
}

func TestCityRewards(t *testing.T) {
	r, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[int][]state.CityReward{
		1: nil,
		2: {state.RewardWorkshop}, // explorer is excluded in MVP
		3: {state.RewardCityWall, state.RewardResources},
		4: {state.RewardPopulationGrowth, state.RewardBorderGrowth},
		5: nil, // past max_city_level: no level-up, no reward
	}
	for level, want := range tests {
		if got := r.City.Rewards(level); !reflect.DeepEqual(got, want) {
			t.Errorf("Rewards(%d) = %v, want %v", level, got, want)
		}
	}
	if got := r.City.LevelUpRewards[2]; !reflect.DeepEqual(got, []state.CityReward{state.RewardWorkshop, state.RewardExplorer}) {
		t.Errorf("LevelUpRewards[2] = %v; the full list keeps excluded rewards", got)
	}
	if !reflect.DeepEqual(r.UnusedInMVP, UnusedInMVP{Buildings: []state.Building{state.Port}, Resources: []state.Resource{state.Fish}}) {
		t.Errorf("UnusedInMVP = %+v", r.UnusedInMVP)
	}
}

// TestX10Conversion checks decimal values convert to exact tenths.
func TestX10Conversion(t *testing.T) {
	tests := []struct {
		text string
		want int
		ok   bool
	}{
		{"0", 0, true}, {"4", 40, true}, {"4.5", 45, true}, {"1.5", 15, true}, {"10", 100, true},
		{"0.1", 1, true}, {"4.0", 40, true},
		{"4.55", 0, false}, {"-1", 0, false}, {"1e1", 0, false}, {".5", 0, false},
		{"04", 0, false}, {"abc", 0, false}, {"", 0, false}, {"4.", 0, false},
	}
	for _, tt := range tests {
		got, ok := decimal{text: tt.text, set: true}.x10()
		if got != tt.want || ok != tt.ok {
			t.Errorf("x10(%q) = %d, %v; want %d, %v", tt.text, got, ok, tt.want, tt.ok)
		}
	}
}

// TestRepoRulesFile loads the real rules/v1.yaml, as the advisor does at
// startup.
func TestRepoRulesFile(t *testing.T) {
	r, err := Load("../../rules/v1.yaml")
	if err != nil {
		t.Fatalf("rules/v1.yaml does not load: %v", err)
	}
	w, ok := r.UnitKind("warrior")
	if !ok {
		t.Fatal("no warrior")
	}
	// Warrior stats and combat constants confirmed in ADR 0001.
	if u := r.Units[w]; u.HPx10 != 100 || u.AttackX10 != 20 || u.DefenseX10 != 20 {
		t.Errorf("warrior = %+v", u)
	}
	if r.Combat.DamageScaleX10 != 45 || r.Combat.DefenseBonusX10 != 15 || r.Veterancy.KillsRequired != 3 {
		t.Errorf("combat %+v, veterancy %+v", r.Combat, r.Veterancy)
	}
	if len(r.Units) != 7 || len(r.Techs) != 19 {
		t.Errorf("%d units and %d techs, want 7 and 19", len(r.Units), len(r.Techs))
	}
	for _, name := range []string{"warrior", "rider", "archer", "defender", "swordsman", "catapult", "knight"} {
		if _, ok := r.UnitKind(name); !ok {
			t.Errorf("MVP unit %s missing", name)
		}
	}
	for l := 2; l <= 4; l++ {
		if len(r.City.LevelUpRewards[l]) != 2 {
			t.Errorf("level %d rewards = %v, want two options", l, r.City.LevelUpRewards[l])
		}
	}
	if !reflect.DeepEqual(r.City.MVPExcludedRewards, []state.CityReward{state.RewardExplorer}) {
		t.Errorf("MVPExcludedRewards = %v, want [explorer]", r.City.MVPExcludedRewards)
	}
	// One city at the start of the game: tier 1 costs 5, tier 3 costs 7.
	if c := r.TechCost; c.Cost(1, 1) != 5 || c.Cost(3, 1) != 7 {
		t.Errorf("tech costs with one city: tier 1 %d, tier 3 %d", c.Cost(1, 1), c.Cost(3, 1))
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("does-not-exist.yaml"); err == nil {
		t.Error("Load of a missing file succeeded")
	}
}

// rejection is one invalid file and a check that Parse's error contains the
// expected typed error.
type rejection struct {
	name  string
	yaml  string
	match func(err error) bool
}

// is returns a check that passes when err contains an error of type T for
// which pred holds (any T, if pred is nil).
func is[T error](pred func(T) bool) func(error) bool {
	return func(err error) bool {
		for _, e := range flatten(err) {
			var target T
			if errors.As(e, &target) && (pred == nil || pred(target)) {
				return true
			}
		}
		return false
	}
}

func TestParseRejects(t *testing.T) {
	cases := []rejection{
		// Unknown fields.
		{"unknown top-level field", edit(t, "version: v1", "verzion: v1"), is(func(e *UnknownFieldError) bool { return e.Field == "verzion" })},
		{"unknown unit field", edit(t, "cost: 2, hp: 10", "cost: 2, hpp: 10"), is(func(e *UnknownFieldError) bool { return e.Field == "hpp" })},
		{"old x10 combat key", edit(t, "wall_bonus: 4", "wall_bonus_x10: 40"), is(func(e *UnknownFieldError) bool { return e.Field == "wall_bonus_x10" })},
		{"unknown tech field", edit(t, "riding:       { tier: 1 }", "riding:       { tier: 1, cost: 5 }"), is[*UnknownFieldError](nil)},

		// Unknown skills.
		{"unknown skill", edit(t, "skills: [dash, persist, fortify]", "skills: [dash, swim]"), is(func(e *UnknownSkillError) bool { return e.Unit == "knight" && e.Skill == "swim" })},

		// Units requiring nonexistent techs, and techs requiring them.
		{"unit needs missing tech", edit(t, "tech: archery", "tech: sailing"), is(func(e *UnknownTechError) bool {
			return e.Path == "units.archer.tech" && e.Tech == "sailing"
		})},
		{"tech requires missing tech", edit(t, "requires: hunting", "requires: magic"), is(func(e *UnknownTechError) bool { return e.Path == "techs.archery.requires" })},

		// Cycles.
		{"two-tech cycle", edit(t, "riding:       { tier: 1 }", "riding:       { tier: 1, requires: free_spirit }"), is(func(e *TechCycleError) bool {
			return reflect.DeepEqual(e.Cycle, []string{"free_spirit", "riding"})
		})},
		{"self cycle", edit(t, "tier: 1, requires: null, unlocks: [harvest_game]", "tier: 1, requires: hunting"), is(func(e *TechCycleError) bool { return reflect.DeepEqual(e.Cycle, []string{"hunting"}) })},

		// Rewards outside the vocabulary.
		{"unknown level-up reward", edit(t, "3: [city_wall, resources]", "3: [city_wall, castle]"), is(func(e *UnknownRewardError) bool {
			return e.Path == "city.level_up_rewards.3" && e.Reward == "castle"
		})},
		{"none is not a reward", edit(t, "3: [city_wall, resources]", "3: [city_wall, none]"), is[*UnknownRewardError](nil)},
		{"unknown excluded reward", edit(t, "mvp_excluded_rewards: [explorer]", "mvp_excluded_rewards: [explorers]"), is(func(e *UnknownRewardError) bool { return e.Path == "city.mvp_excluded_rewards" })},

		// Missing or placeholder game_build.
		{"missing game_build", edit(t, `game_build: "Android 2.17.3.16375"`, ""), is[*GameBuildError](nil)},
		{"blank game_build", edit(t, `"Android 2.17.3.16375"`, `"  "`), is[*GameBuildError](nil)},
		{"design doc placeholder", edit(t, `"Android 2.17.3.16375"`, `"<record at Phase 1 start; see Pinning the game build>"`), is[*GameBuildError](nil)},
		{"TBD game_build", edit(t, `"Android 2.17.3.16375"`, `"TBD"`), is[*GameBuildError](nil)},

		// Malformed YAML.
		{"duplicate unit", edit(t, "units:\n", "units:\n  archer: { cost: 1 }\n"), is(func(e *DecodeError) bool { return strings.Contains(e.Msg, "already defined") })},
		{"string for int", edit(t, "kills_required: 3", "kills_required: three"), is[*DecodeError](nil)},
		{"list for decimal", edit(t, "damage_scale: 4.5", "damage_scale: [4.5]"), is[*DecodeError](nil)},
		{"syntax error", edit(t, "combat:", "combat: ["), is[*DecodeError](nil)},
		{"empty file", "", is[*DecodeError](nil)},
		{"two documents", validYAML + "---\nversion: v2\n", is[*DecodeError](nil)},

		// Values out of range or inconsistent.
		{"too precise for x10", edit(t, "attack: 3.5", "attack: 3.55"), is(func(e *InvalidValueError) bool { return e.Path == "units.knight.attack" })},
		{"negative hp", edit(t, "cost: 2, hp: 10", "cost: 2, hp: -1"), is[*InvalidValueError](nil)},
		{"zero hp", edit(t, "cost: 2, hp: 10", "cost: 2, hp: 0"), is[*InvalidValueError](nil)},
		{"missing hp", edit(t, "cost: 2, hp: 10, ", "cost: 2, "), is(func(e *InvalidValueError) bool { return e.Path == "units.warrior.hp" })},
		{"hp overflows int16", edit(t, "cost: 2, hp: 10", "cost: 2, hp: 5000"), is[*InvalidValueError](nil)},
		{"zero cost", edit(t, "cost: 2,", "cost: 0,"), is[*InvalidValueError](nil)},
		{"zero move", edit(t, "move: 3", "move: 0"), is[*InvalidValueError](nil)},
		{"zero range", edit(t, "range: 2", "range: 0"), is[*InvalidValueError](nil)},
		{"duplicate skill", edit(t, "skills: [dash, persist, fortify]", "skills: [dash, dash]"), is[*InvalidValueError](nil)},
		{"bad unit name", edit(t, "knight:", "Knight:"), is[*InvalidValueError](nil)},
		{"no units", noUnits(t), is(func(e *InvalidValueError) bool { return e.Path == "units" })},
		{"wrong tier", edit(t, "tier: 3, requires: free_spirit", "tier: 2, requires: free_spirit"), is(func(e *InvalidValueError) bool { return e.Path == "techs.chivalry.tier" })},
		{"root not tier 1", edit(t, "riding:       { tier: 1 }", "riding:       { tier: 2 }"), is[*InvalidValueError](nil)},
		{"duplicate unlock", edit(t, "unlocks: [forest_defense]", "unlocks: [forest_defense, forest_defense]"), is[*InvalidValueError](nil)},
		{"missing tech_cost base", edit(t, "  base: 4\n", ""), is(func(e *InvalidValueError) bool { return e.Path == "tech_cost.base" })},
		{"zero tech cost per tier", edit(t, "per_tier_per_city: 1", "per_tier_per_city: 0"), is[*InvalidValueError](nil)},
		{"missing hp_bonus", edit(t, "  hp_bonus: 5\n", ""), is(func(e *InvalidValueError) bool { return e.Path == "veterancy.hp_bonus" })},
		{"veteran hp overflows", edit(t, "hp_bonus: 5", "hp_bonus: 3270"), is(func(e *InvalidValueError) bool { return strings.HasSuffix(e.Path, ".hp") })},
		{"zero kills_required", edit(t, "kills_required: 3", "kills_required: 0"), is[*InvalidValueError](nil)},
		{"max_city_level too low", edit(t, "max_city_level: 4", "max_city_level: 1"), is[*InvalidValueError](nil)},
		{"level above max", edit(t, "max_city_level: 4", "max_city_level: 3"), is(func(e *InvalidValueError) bool { return e.Path == "city.level_up_rewards.4" })},
		{"level missing", edit(t, "    3: [city_wall, resources]\n", ""), is(func(e *InvalidValueError) bool { return e.Path == "city.level_up_rewards.3" })},
		{"every option excluded", edit(t, "mvp_excluded_rewards: [explorer]", "mvp_excluded_rewards: [explorer, workshop]"), is(func(e *InvalidValueError) bool { return e.Path == "city.level_up_rewards.2" })},
		{"duplicate reward", edit(t, "3: [city_wall, resources]", "3: [city_wall, city_wall]"), is[*InvalidValueError](nil)},
		{"unknown building", edit(t, "buildings: [port]", "buildings: [harbour]"), is[*InvalidValueError](nil)},
		{"unknown resource", edit(t, "resources: [fish]", "resources: [whales]"), is[*InvalidValueError](nil)},
		{"unknown formula", edit(t, "formula: community_v1", "formula: community_v2"), is[*InvalidValueError](nil)},
		{"missing version", edit(t, "version: v1\n", ""), is[*InvalidValueError](nil)},
		{"defense bonus below 1", edit(t, "defense_bonus: 1.5", "defense_bonus: 0.5"), is[*InvalidValueError](nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("Parse succeeded: %+v", r)
			}
			if r != nil {
				t.Error("Parse returned rules along with an error")
			}
			if !tc.match(err) {
				t.Errorf("error does not include the expected typed error:\n%v", err)
			}
		})
	}
}

// flatten unwraps errors.Join trees into their leaves.
func flatten(err error) []error {
	var j interface{ Unwrap() []error }
	if errors.As(err, &j) {
		var out []error
		for _, e := range j.Unwrap() {
			out = append(out, flatten(e)...)
		}
		return out
	}
	return []error{err}
}

// TestParseReportsAllErrors checks that one run shows every problem, not
// just the first.
func TestParseReportsAllErrors(t *testing.T) {
	y := strings.Replace(validYAML, "skills: [dash, persist, fortify]", "skills: [swim]", 1)
	y = strings.Replace(y, `"Android 2.17.3.16375"`, `"TODO"`, 1)
	y = strings.Replace(y, "tech: archery", "tech: sailing", 1)
	_, err := Parse([]byte(y))
	var (
		skill *UnknownSkillError
		build *GameBuildError
		tech  *UnknownTechError
	)
	if !errors.As(err, &skill) || !errors.As(err, &build) || !errors.As(err, &tech) {
		t.Errorf("want all three errors, got:\n%v", err)
	}
}

// TestErrorMessages makes sure every error type renders a useful message.
func TestErrorMessages(t *testing.T) {
	errs := []error{
		&DecodeError{Line: 3, Msg: "bad"}, &DecodeError{Msg: "rules file is empty"},
		&UnknownFieldError{Line: 2, Field: "hpp", In: "rules.unitYAML"},
		&GameBuildError{}, &GameBuildError{Value: "<x>"},
		&UnknownSkillError{Unit: "w", Skill: "swim"},
		&UnknownTechError{Path: "units.a.tech", Tech: "x"},
		&TechCycleError{Cycle: []string{"a", "b"}},
		&UnknownRewardError{Path: "city.x", Reward: "castle"},
		&InvalidValueError{Path: "p", Value: "v", Reason: "is bad"},
	}
	for _, e := range errs {
		if msg := e.Error(); !strings.HasPrefix(msg, "rules: ") || len(msg) < 15 {
			t.Errorf("%T message %q", e, msg)
		}
	}
	if got := fmt.Sprint(&TechCycleError{Cycle: []string{"a", "b"}}); got != "rules: tech tree cycle: a -> b -> a" {
		t.Errorf("cycle message %q", got)
	}
}

// TestRepoRulesFileMarksEveryValue enforces the rules file's convention that
// every value line says whether it was observed in-game, still needs
// verifying, or is an MVP design decision.
func TestRepoRulesFileMarksEveryValue(t *testing.T) {
	data, err := os.ReadFile("../../rules/v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{"# confirmed", "# verify", "# design decision"}
	for i, line := range strings.Split(string(data), "\n") {
		content, _, _ := strings.Cut(line, "#")
		key, value, found := strings.Cut(content, ":")
		key = strings.TrimSpace(key)
		if !found || strings.TrimSpace(value) == "" || key == "version" {
			continue // blank, comment, section header, or the schema version
		}
		marked := false
		for _, m := range markers {
			marked = marked || strings.Contains(line, m)
		}
		if !marked {
			t.Errorf("rules/v1.yaml:%d: %q has no # confirmed, # verify or # design decision comment", i+1, strings.TrimSpace(line))
		}
	}
}
