package data_structure

import "math"

// SortedSet stores unique members with scores, ordered by score and member.
type SortedSet struct {
	scores map[string]float64
	order  *SkipList
}

// NewSortedSet creates an empty Sorted Set.
func NewSortedSet() *SortedSet {
	return &SortedSet{
		scores: make(map[string]float64),
		order:  NewSkipList(),
	}
}

// Add inserts a new member or updates its score.
// It returns true only when the member is newly added.
func (ss *SortedSet) Add(score float64, member string) bool {
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return false
	}

	oldScore, exists := ss.scores[member]
	if exists {
		if oldScore == score {
			return false
		}

		ss.order.Remove(oldScore, member)
	}

	ss.scores[member] = score
	ss.order.Insert(score, member)

	return !exists
}

// Score returns the score associated with member.
func (ss *SortedSet) Score(member string) (float64, bool) {
	score, exists := ss.scores[member]
	return score, exists
}

// Rank returns the zero-based rank of member.
func (ss *SortedSet) Rank(member string) (int, bool) {
	score, exists := ss.scores[member]
	if !exists {
		return 0, false
	}

	return ss.order.Rank(score, member)
}

// Remove deletes member from the Sorted Set.
// It returns true if the member existed.
func (ss *SortedSet) Remove(member string) bool {
	score, exists := ss.scores[member]
	if !exists {
		return false
	}

	if !ss.order.Remove(score, member) {
		return false
	}

	delete(ss.scores, member)
	return true
}

// Len returns the number of members.
func (ss *SortedSet) Len() int {
	return len(ss.scores)
}
