package data_structure

import "math/rand"

const skipListMaxLevel = 32

type skipListNode struct {
	member  string
	score   float64
	forward []*skipListNode
}

// SkipList stores members ordered by score, then lexicographically by member.
type SkipList struct {
	head  *skipListNode
	level int
}

// NewSkipList creates an empty Skip List.
func NewSkipList() *SkipList {
	return &SkipList{
		head:  &skipListNode{forward: make([]*skipListNode, skipListMaxLevel)},
		level: 1,
	}
}

func less(scoreA float64, memberA string, scoreB float64, memberB string) bool {
	if scoreA != scoreB {
		return scoreA < scoreB
	}
	return memberA < memberB
}

func randomLevel() int {
	level := 1
	for level < skipListMaxLevel && rand.Intn(4) == 0 {
		level++
	}
	return level
}

// Search returns true if the (score, member) pair exists.
func (sl *SkipList) Search(score float64, member string) bool {
	cur := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil &&
			less(cur.forward[i].score, cur.forward[i].member, score, member) {
			cur = cur.forward[i]
		}
	}

	cur = cur.forward[0]
	return cur != nil && cur.score == score && cur.member == member
}

// Insert adds a (score, member) pair.
// Callers should ensure that the pair does not already exist.
func (sl *SkipList) Insert(score float64, member string) {
	update := make([]*skipListNode, skipListMaxLevel)
	cur := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil &&
			less(cur.forward[i].score, cur.forward[i].member, score, member) {
			cur = cur.forward[i]
		}
		update[i] = cur
	}

	level := randomLevel()
	if level > sl.level {
		for i := sl.level; i < level; i++ {
			update[i] = sl.head
		}
		sl.level = level
	}

	node := &skipListNode{
		member:  member,
		score:   score,
		forward: make([]*skipListNode, level),
	}

	for i := 0; i < level; i++ {
		node.forward[i] = update[i].forward[i]
		update[i].forward[i] = node
	}
}

// Remove deletes a (score, member) pair.
// It returns true if the pair existed.
func (sl *SkipList) Remove(score float64, member string) bool {
	update := make([]*skipListNode, skipListMaxLevel)
	cur := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil &&
			less(cur.forward[i].score, cur.forward[i].member, score, member) {
			cur = cur.forward[i]
		}
		update[i] = cur
	}

	target := cur.forward[0]
	if target == nil || target.score != score || target.member != member {
		return false
	}

	for i := 0; i < sl.level; i++ {
		if update[i].forward[i] == target {
			update[i].forward[i] = target.forward[i]
		}
	}

	for sl.level > 1 && sl.head.forward[sl.level-1] == nil {
		sl.level--
	}

	return true
}

// Rank returns the zero-based rank of a pair.
// This implementation traverses level 0, so its time complexity is O(N).
func (sl *SkipList) Rank(score float64, member string) (int, bool) {
	rank := 0

	for cur := sl.head.forward[0]; cur != nil; cur = cur.forward[0] {
		if cur.score == score && cur.member == member {
			return rank, true
		}
		rank++
	}

	return 0, false
}

// Len returns the number of nodes in the Skip List.
// This traversal is O(N).
func (sl *SkipList) Len() int {
	count := 0
	for cur := sl.head.forward[0]; cur != nil; cur = cur.forward[0] {
		count++
	}
	return count
}
