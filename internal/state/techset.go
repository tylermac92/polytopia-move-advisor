package state

import "math/bits"

// TechID indexes the techs defined in the rules file. It must be below
// MaxTechs.
type TechID uint8

// MaxTechs is the number of techs a TechSet can hold.
const MaxTechs = 64

// TechSet is a set of researched techs, stored as a bitset. It is a value
// type (a single integer, never a slice or map) so copying a Player copies
// its techs, which keeps GameState.Clone a flat copy.
type TechSet uint64

// Has reports whether t is in the set.
func (s TechSet) Has(t TechID) bool { return s&(1<<t) != 0 }

// With returns the set with t added.
func (s TechSet) With(t TechID) TechSet { return s | 1<<t }

// Without returns the set with t removed.
func (s TechSet) Without(t TechID) TechSet { return s &^ (1 << t) }

// Len returns the number of techs in the set.
func (s TechSet) Len() int { return bits.OnesCount64(uint64(s)) }
