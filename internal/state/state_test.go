package state

import "testing"

func TestAllocUnitIDPerPlayer(t *testing.T) {
	s := &GameState{Players: []Player{{ID: 0}, {ID: 1}}}
	p0, p1 := &s.Players[0], &s.Players[1]

	if got := p0.AllocUnitID(); got != 1 {
		t.Fatalf("first ID = %d, want 1", got)
	}
	// Player 1 training many units must not move player 0's counter, or the
	// gap in player 0's own IDs would reveal player 1's training count.
	for i := 0; i < 5; i++ {
		p1.AllocUnitID()
	}
	if got := p0.AllocUnitID(); got != 2 {
		t.Errorf("player 0 second ID = %d, want 2 regardless of player 1", got)
	}
	if got := p1.AllocUnitID(); got != 6 {
		t.Errorf("player 1 sixth ID = %d, want 6", got)
	}
}

func TestUnitIndexUsesOwnerAndID(t *testing.T) {
	s := &GameState{Units: []Unit{
		{ID: 1, Owner: 0},
		{ID: 1, Owner: 1},
		{ID: 2, Owner: 1},
	}}
	tests := []struct {
		ref  UnitRef
		want int
	}{
		{UnitRef{Owner: 0, ID: 1}, 0},
		{UnitRef{Owner: 1, ID: 1}, 1},
		{UnitRef{Owner: 1, ID: 2}, 2},
		{UnitRef{Owner: 0, ID: 2}, -1},
		{UnitRef{Owner: 0, ID: NoUnit}, -1},
	}
	for _, tt := range tests {
		if got := s.UnitIndex(tt.ref); got != tt.want {
			t.Errorf("UnitIndex(%+v) = %d, want %d", tt.ref, got, tt.want)
		}
	}
	if got := s.Units[1].Ref(); got != (UnitRef{Owner: 1, ID: 1}) {
		t.Errorf("Ref() = %+v", got)
	}
}

func TestTechSet(t *testing.T) {
	var s TechSet
	s = s.With(0).With(5).With(MaxTechs - 1)
	for _, id := range []TechID{0, 5, MaxTechs - 1} {
		if !s.Has(id) {
			t.Errorf("Has(%d) = false after With", id)
		}
	}
	if s.Has(1) {
		t.Error("Has(1) = true, never added")
	}
	if s.Len() != 3 {
		t.Errorf("Len() = %d, want 3", s.Len())
	}
	s2 := s.Without(5)
	if s2.Has(5) || !s.Has(5) {
		t.Error("Without must remove from the result only; TechSet is a value")
	}
}
