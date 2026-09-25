package rules

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tylermac92/polytopia-move-advisor/internal/state"
)

// Load reads and validates the rules file at path.
func Load(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	r, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s:\n%w", path, err)
	}
	return r, nil
}

// Parse decodes and validates a rules file. Decoding is strict: unknown
// fields and duplicate keys are errors. On failure it returns every problem
// found, joined with errors.Join; each is one of the typed errors in
// errors.go, so callers test with errors.As.
func Parse(data []byte) (*Rules, error) {
	f, err := decode(data)
	if err != nil {
		return nil, err
	}
	v := &validator{}
	r := v.build(f)
	if err := errors.Join(v.errs...); err != nil {
		return nil, err
	}
	return r, nil
}

var (
	unknownFieldMsg = regexp.MustCompile(`^line (\d+): field (\S+) not found in type (\S+)$`)
	lineMsg         = regexp.MustCompile(`^(?:yaml: )?line (\d+): (.*)$`)
)

func decode(data []byte) (*fileYAML, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f fileYAML
	err := dec.Decode(&f)
	if errors.Is(err, io.EOF) {
		return nil, &DecodeError{Msg: "rules file is empty"}
	}
	if err != nil {
		return nil, decodeErrors(err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, &DecodeError{Msg: "rules file must hold exactly one YAML document"}
	}
	return &f, nil
}

// decodeErrors converts a YAML error into typed errors, one per message.
func decodeErrors(err error) error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return lineError(err.Error())
	}
	errs := make([]error, 0, len(te.Errors))
	for _, msg := range te.Errors {
		if m := unknownFieldMsg.FindStringSubmatch(msg); m != nil {
			line, _ := strconv.Atoi(m[1])
			errs = append(errs, &UnknownFieldError{Line: line, Field: m[2], In: m[3]})
			continue
		}
		errs = append(errs, lineError(msg))
	}
	return errors.Join(errs...)
}

func lineError(msg string) *DecodeError {
	if m := lineMsg.FindStringSubmatch(msg); m != nil {
		line, _ := strconv.Atoi(m[1])
		return &DecodeError{Line: line, Msg: m[2]}
	}
	return &DecodeError{Msg: strings.TrimPrefix(msg, "yaml: ")}
}

// validator collects every validation error rather than stopping at the
// first, so one run shows everything wrong with the file.
type validator struct {
	errs []error
}

func (v *validator) add(err error) { v.errs = append(v.errs, err) }

func (v *validator) invalid(path string, value any, reason string) {
	v.add(&InvalidValueError{Path: path, Value: fmt.Sprint(value), Reason: reason})
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// placeholderPattern matches game_build values left from a template, such as
// the design doc's "<record at Phase 1 start; ...>".
var placeholderPattern = regexp.MustCompile(`(?i)[<>]|\b(todo|tbd|fixme|placeholder|unknown)\b`)

func (v *validator) build(f *fileYAML) *Rules {
	r := &Rules{Version: f.Version, GameBuild: f.GameBuild}
	if strings.TrimSpace(f.Version) == "" {
		v.invalid("version", f.Version, "must be set")
	}
	if strings.TrimSpace(f.GameBuild) == "" || placeholderPattern.MatchString(f.GameBuild) {
		v.add(&GameBuildError{Value: f.GameBuild})
	}
	v.buildTechs(r, f.Techs)
	v.buildUnits(r, f.Units)
	v.buildVeterancy(r, f.Veterancy)
	v.buildCity(r, f.City)
	v.buildUnused(r, f.UnusedInMVP)
	v.buildCombat(r, f.Combat)
	return r
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (v *validator) buildTechs(r *Rules, techs map[string]techYAML) {
	names := sortedKeys(techs)
	if len(names) > state.MaxTechs {
		v.invalid("techs", len(names), fmt.Sprintf("techs defined; a TechSet holds at most %d", state.MaxTechs))
		return
	}
	r.techByName = make(map[string]state.TechID, len(names))
	for i, name := range names {
		r.techByName[name] = state.TechID(i)
	}
	r.Techs = make([]Tech, len(names))
	for i, name := range names {
		t := techs[name]
		path := "techs." + name
		if !namePattern.MatchString(name) {
			v.invalid(path, name, "is not a valid name (lowercase letters, digits and _)")
		}
		out := Tech{Name: name, Tier: t.Tier, Requires: NoTech}
		if t.Requires != nil {
			if id, ok := r.techByName[*t.Requires]; ok {
				out.Requires = id
			} else {
				v.add(&UnknownTechError{Path: path + ".requires", Tech: *t.Requires})
			}
		}
		out.Unlocks = v.uniqueNames(path+".unlocks", t.Unlocks)
		r.Techs[i] = out
	}
	if v.checkTechCycles(r) {
		v.checkTiers(r, techs)
	}
}

// checkTechCycles reports each cycle in the requires links once, and returns
// true if there are none. Each tech has at most one parent, so following
// parents from any tech either reaches a root or enters a cycle.
func (v *validator) checkTechCycles(r *Rules) bool {
	const (
		unvisited = iota
		onPath
		done
	)
	mark := make([]int, len(r.Techs))
	ok := true
	for start := range r.Techs {
		var path []int
		t := start
		for t != int(NoTech) && mark[t] == unvisited {
			mark[t] = onPath
			path = append(path, t)
			t = int(r.Techs[t].Requires)
		}
		if t != int(NoTech) && mark[t] == onPath {
			// t is on the current path, so the path from t onward is a cycle.
			i := slices.Index(path, t)
			v.add(&TechCycleError{Cycle: cycleNames(r, path[i:])})
			ok = false
		}
		for _, p := range path {
			mark[p] = done
		}
	}
	return ok
}

// cycleNames rotates a cycle to start at its smallest name, so the error is
// the same whichever tech the search entered it from.
func cycleNames(r *Rules, cycle []int) []string {
	names := make([]string, len(cycle))
	for i, t := range cycle {
		names[i] = r.Techs[t].Name
	}
	m := slices.Index(names, slices.Min(names))
	return append(names[m:], names[:m]...)
}

// checkTiers requires tier 1 for roots and parent tier + 1 otherwise. It runs
// only on an acyclic tree.
func (v *validator) checkTiers(r *Rules, techs map[string]techYAML) {
	for _, t := range r.Techs {
		want := 1
		if t.Requires != NoTech {
			want = r.Techs[t.Requires].Tier + 1
		}
		if t.Tier != want {
			req := "no requires"
			if raw := techs[t.Name].Requires; raw != nil {
				req = "requires " + *raw
			}
			v.invalid("techs."+t.Name+".tier", t.Tier, fmt.Sprintf("should be %d (%s)", want, req))
		}
	}
}

func (v *validator) buildUnits(r *Rules, units map[string]unitYAML) {
	names := sortedKeys(units)
	switch {
	case len(names) == 0:
		v.invalid("units", 0, "units defined; need at least one")
		return
	case len(names) > math.MaxUint8+1:
		v.invalid("units", len(names), "units defined; UnitKind holds at most 256")
		return
	}
	r.unitByName = make(map[string]state.UnitKind, len(names))
	r.Units = make([]Unit, len(names))
	for i, name := range names {
		u := units[name]
		path := "units." + name
		r.unitByName[name] = state.UnitKind(i)
		if !namePattern.MatchString(name) {
			v.invalid(path, name, "is not a valid name (lowercase letters, digits and _)")
		}
		out := Unit{
			Name:       name,
			Cost:       u.Cost,
			AttackX10:  v.x10(path+".attack", u.Attack, 0),
			DefenseX10: v.x10(path+".defense", u.Defense, 1),
			Move:       u.Move,
			Range:      u.Range,
			Tech:       NoTech,
		}
		// Max HP must fit state.Unit.HP, an int16 in x10.
		hp := v.x10(path+".hp", u.HP, 1)
		if hp > math.MaxInt16 {
			v.invalid(path+".hp", u.HP.text, "is too large")
		}
		out.HPx10 = int16(min(hp, math.MaxInt16))
		if u.Cost < 1 {
			v.invalid(path+".cost", u.Cost, "must be at least 1")
		}
		if u.Move < 1 {
			v.invalid(path+".move", u.Move, "must be at least 1")
		}
		if u.Range < 1 {
			v.invalid(path+".range", u.Range, "must be at least 1")
		}
		if u.Tech != nil {
			if id, ok := r.techByName[*u.Tech]; ok {
				out.Tech = id
			} else {
				v.add(&UnknownTechError{Path: path + ".tech", Tech: *u.Tech})
			}
		}
		for _, s := range u.Skills {
			k := slices.Index(skillNames[:], s)
			switch {
			case k < 0:
				v.add(&UnknownSkillError{Unit: name, Skill: s})
			case out.Skills.Has(Skill(k)):
				v.invalid(path+".skills", s, "is listed twice")
			default:
				out.Skills |= 1 << k
			}
		}
		r.Units[i] = out
	}
}

// x10 converts a required decimal to tenths, reporting a missing or malformed
// value or one below minX10 (in tenths).
func (v *validator) x10(path string, d decimal, minX10 int) int {
	if !d.set {
		v.invalid(path, "", "is required")
		return 0
	}
	n, ok := d.x10()
	if !ok {
		v.invalid(path, d.text, "must be a non-negative number with at most one decimal place")
		return 0
	}
	if n < minX10 {
		v.invalid(path, d.text, fmt.Sprintf("must be at least %g", float64(minX10)/10))
	}
	return n
}

func (v *validator) buildVeterancy(r *Rules, vet veterancyYAML) {
	r.Veterancy.KillsRequired = vet.KillsRequired
	// state.Unit.Kills is an int8.
	if vet.KillsRequired < 1 || vet.KillsRequired > math.MaxInt8 {
		v.invalid("veterancy.kills_required", vet.KillsRequired, fmt.Sprintf("must be between 1 and %d", math.MaxInt8))
	}
}

func (v *validator) buildCity(r *Rules, c cityYAML) {
	maxLevel := c.MaxCityLevel
	r.City.MaxCityLevel = maxLevel
	// state.City.Level is an int8.
	if maxLevel < 2 || maxLevel > math.MaxInt8 {
		v.invalid("city.max_city_level", maxLevel, fmt.Sprintf("must be between 2 and %d", math.MaxInt8))
		return
	}
	r.City.MVPExcludedRewards = v.rewards("city.mvp_excluded_rewards", c.MVPExcludedRewards)
	r.City.LevelUpRewards = make([][]state.CityReward, maxLevel+1)
	levels := make([]int, 0, len(c.LevelUpRewards))
	for l := range c.LevelUpRewards {
		levels = append(levels, l)
	}
	slices.Sort(levels)
	for _, l := range levels {
		path := fmt.Sprintf("city.level_up_rewards.%d", l)
		if l < 2 || l > maxLevel {
			v.invalid(path, l, fmt.Sprintf("is not a level-up (levels 2 to max_city_level %d)", maxLevel))
			continue
		}
		r.City.LevelUpRewards[l] = v.rewards(path, c.LevelUpRewards[l])
	}
	// Every reachable level must offer a reward the MVP can pick, or a city
	// reaching it would have a PendingReward with no legal CityUpgrade.
	for l := 2; l <= maxLevel; l++ {
		if _, ok := c.LevelUpRewards[l]; ok && len(r.City.Rewards(l)) == 0 {
			v.invalid(fmt.Sprintf("city.level_up_rewards.%d", l), c.LevelUpRewards[l], "offers no reward allowed in MVP play")
		} else if !ok {
			v.invalid(fmt.Sprintf("city.level_up_rewards.%d", l), "", "is missing; every level up to max_city_level needs rewards")
		}
	}
}

// rewards parses reward names, rejecting unknown and duplicate ones.
func (v *validator) rewards(path string, names []string) []state.CityReward {
	out := make([]state.CityReward, 0, len(names))
	for _, n := range names {
		var rw state.CityReward
		if err := rw.UnmarshalText([]byte(n)); err != nil || rw == state.RewardNone {
			v.add(&UnknownRewardError{Path: path, Reward: n})
			continue
		}
		if containsReward(out, rw) {
			v.invalid(path, n, "is listed twice")
			continue
		}
		out = append(out, rw)
	}
	return out
}

func (v *validator) buildUnused(r *Rules, u unusedYAML) {
	for _, n := range u.Buildings {
		var b state.Building
		if err := b.UnmarshalText([]byte(n)); err != nil || b == state.BuildingNone {
			v.invalid("unused_in_mvp.buildings", n, "is not a known building")
			continue
		}
		r.UnusedInMVP.Buildings = append(r.UnusedInMVP.Buildings, b)
	}
	for _, n := range u.Resources {
		var res state.Resource
		if err := res.UnmarshalText([]byte(n)); err != nil || res == state.ResourceNone {
			v.invalid("unused_in_mvp.resources", n, "is not a known resource")
			continue
		}
		r.UnusedInMVP.Resources = append(r.UnusedInMVP.Resources, res)
	}
}

func (v *validator) buildCombat(r *Rules, c combatYAML) {
	r.Combat = Combat{
		Formula:         c.Formula,
		DamageScaleX10:  v.x10("combat.damage_scale", c.DamageScale, 1),
		DefenseBonusX10: v.x10("combat.defense_bonus", c.DefenseBonus, 10),
		WallBonusX10:    v.x10("combat.wall_bonus", c.WallBonus, 10),
	}
	if c.Formula != FormulaCommunityV1 {
		v.invalid("combat.formula", c.Formula, "is not a known formula (known: "+FormulaCommunityV1+")")
	}
}

// uniqueNames checks a list of free-form names for syntax and duplicates.
func (v *validator) uniqueNames(path string, names []string) []string {
	for i, n := range names {
		if !namePattern.MatchString(n) {
			v.invalid(path, n, "is not a valid name (lowercase letters, digits and _)")
		} else if slices.Contains(names[:i], n) {
			v.invalid(path, n, "is listed twice")
		}
	}
	return names
}
