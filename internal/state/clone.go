package state

func cloneSlice[T any](src []T) []T {
	if src == nil {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

// Clone returns a deep copy of s that shares no memory with it.
//
// A struct copy alone would share the slices' backing arrays, so a clone
// would mutate its parent; each slice is therefore copied. Slice elements
// hold no slices, maps or pointers, so copying one level is enough. Any
// slice added to GameState must be copied here too; TestCloneCopiesEverySlice
// fails otherwise.
func (s *GameState) Clone() *GameState {
	c := *s // copies scalars; slice headers still alias s until replaced below
	c.Tiles = cloneSlice(s.Tiles)
	c.Units = cloneSlice(s.Units)
	c.Cities = cloneSlice(s.Cities)
	c.Players = cloneSlice(s.Players)
	return &c
}
