package data_structure

import "sync"

// Set stores unique string members.
type Set struct {
	mu      sync.RWMutex
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.members[member]; exists {
		return false
	}

	s.members[member] = struct{}{}
	return true
}

// Remove deletes member from the Set.
// It returns true if the member existed.
func (s *Set) Remove(member string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.members[member]; !exists {
		return false
	}

	delete(s.members, member)
	return true
}

// Contains reports whether member exists in the Set.
func (s *Set) Contains(member string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.members[member]
	return exists
}

// Members returns a snapshot of all members.
// The order of members is unspecified.
func (s *Set) Members() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members := make([]string, 0, len(s.members))
	for member := range s.members {
		members = append(members, member)
	}
	return members
}

// Len returns the number of members.
func (s *Set) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.members)
}
