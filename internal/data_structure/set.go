package data_structure

// Set stores unique string members.
type Set struct {
	members map[string]struct{}
}

// NewSet creates an empty Set.
func NewSet() *Set {
	return &Set{
		members: make(map[string]struct{}),
	}
}

// Add inserts member into the Set.
// It returns true if the member did not previously exist.
func (s *Set) Add(member string) bool {
	if _, exists := s.members[member]; exists {
		return false
	}

	s.members[member] = struct{}{}
	return true
}

// Remove deletes member from the Set.
// It returns true if the member existed.
func (s *Set) Remove(member string) bool {
	if _, exists := s.members[member]; !exists {
		return false
	}

	delete(s.members, member)
	return true
}

// Contains reports whether member exists in the Set.
func (s *Set) Contains(member string) bool {
	_, exists := s.members[member]
	return exists
}

// Members returns a snapshot of all members.
// The order of members is unspecified.
func (s *Set) Members() []string {
	members := make([]string, 0, len(s.members))
	for member := range s.members {
		members = append(members, member)
	}
	return members
}

// Len returns the number of members.
func (s *Set) Len() int {
	return len(s.members)
}
