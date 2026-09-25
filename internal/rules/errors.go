package rules

import (
	"fmt"
	"strings"
)

// Validation errors. Load returns every problem it finds, joined with
// errors.Join, so use errors.As to test for a specific type.

// DecodeError is a YAML syntax or type error other than an unknown field,
// such as a string where a number belongs or a duplicate key.
type DecodeError struct {
	Line int // 0 if unknown
	Msg  string
}

func (e *DecodeError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("rules: line %d: %s", e.Line, e.Msg)
	}
	return "rules: " + e.Msg
}

// UnknownFieldError is a key the schema doesn't define, usually a typo.
type UnknownFieldError struct {
	Line  int
	Field string
	In    string // Go type the field was found in, such as "rules.unitYAML"
}

func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("rules: line %d: unknown field %q in %s", e.Line, e.Field, e.In)
}

// GameBuildError means game_build is missing, blank or still a placeholder.
// The scenario runner compares it against each scenario's build, so it must
// be the real version string read from the game.
type GameBuildError struct {
	Value string
}

func (e *GameBuildError) Error() string {
	if strings.TrimSpace(e.Value) == "" {
		return "rules: game_build is missing"
	}
	return fmt.Sprintf("rules: game_build %q is a placeholder; record the real build (see Pinning the game build)", e.Value)
}

// UnknownSkillError is a unit skill outside the MVP vocabulary.
type UnknownSkillError struct {
	Unit, Skill string
}

func (e *UnknownSkillError) Error() string {
	return fmt.Sprintf("rules: units.%s: unknown skill %q (known: %s)", e.Unit, e.Skill, strings.Join(skillNames[:], ", "))
}

// UnknownTechError is a reference to a tech the file doesn't define, from a
// unit's tech or a tech's requires.
type UnknownTechError struct {
	Path string // such as "units.archer.tech"
	Tech string
}

func (e *UnknownTechError) Error() string {
	return fmt.Sprintf("rules: %s: unknown tech %q", e.Path, e.Tech)
}

// TechCycleError is a cycle in the tech tree's requires links.
type TechCycleError struct {
	Cycle []string // techs in requires order, starting from the smallest name
}

func (e *TechCycleError) Error() string {
	return fmt.Sprintf("rules: tech tree cycle: %s -> %s", strings.Join(e.Cycle, " -> "), e.Cycle[0])
}

// UnknownRewardError is a city reward outside the known vocabulary.
type UnknownRewardError struct {
	Path   string // such as "city.level_up_rewards.3"
	Reward string
}

func (e *UnknownRewardError) Error() string {
	return fmt.Sprintf("rules: %s: unknown reward %q", e.Path, e.Reward)
}

// InvalidValueError is a value that is present but out of range, malformed,
// duplicated or inconsistent with the rest of the file.
type InvalidValueError struct {
	Path   string
	Value  string
	Reason string
}

func (e *InvalidValueError) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("rules: %s %s", e.Path, e.Reason)
	}
	return fmt.Sprintf("rules: %s: %q %s", e.Path, e.Value, e.Reason)
}
