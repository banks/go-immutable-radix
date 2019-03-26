package art

import (
	"sort"
)

// node16 is a radix node with 5-16 children. We implement binary search over
// the index. The original paper and C versions optimise this further with SIMD
// intrinsics to perform the comparison on all 16 bytes at once however
// generating SIMD assembly for Go is non-trivial and left for a later time.
type node16 struct {
	innerNodeHeader
	index    [16]byte
	children [16]*nodeHeader
}

// index returns the child index of the child with the next byte c. If there is
// no such child, -1 is returned.
func (n *node16) indexOf(c byte) int {
	idx := sort.Search(int(n.nChildren), func(i int) bool {
		return n.index[i] >= c
	})
	if idx < int(n.nChildren) && n.index[idx] == c {
		return idx
	}
	return -1
}

// findChild returns the child with the given next byte if any exists or nil.
func (n *node16) findChild(c byte) *nodeHeader {
	if idx := n.indexOf(c); idx > -1 {
		return n.children[idx]
	}
	return nil
}

// addChild adds the child to the current node16 in place if possible or copies
// itself into a node16 and returns that. We assume there is no existing child
// with the same next byte. This MUST be ensured by the caller. Since the caller
// always knows in practice it's cheaper not to check again here.
func (n *node16) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	if n.nChildren < 16 {
		// Fast path, we have space so update in place
		// Find the right place to insert
		idx := sort.Search(int(n.nChildren), func(i int) bool {
			return n.index[i] >= c
		})
		insertIndex(n.index[0:n.nChildren], c, idx)
		insertChild(n.children[0:n.nChildren], child, idx)
		n.nChildren++
		return &n.nodeHeader
	}

	// Need to grow to a node48
	n48 := txn.newNode48()

	// Copy prefix
	n48.prefixLen = n.prefixLen
	copy(n48.prefix[0:maxPrefixLen], n.prefix[0:maxPrefixLen])

	// Copy children
	for childIdx, childC := range n.index {
		idx := int(n48.nChildren)
		n48.index[childC] = byte(idx + 1)
		n48.children[idx] = n.children[childIdx]
		n48.nChildren++
	}
	// Add new child
	n48.index[c] = byte(n48.nChildren + 1)
	n48.children[n48.nChildren] = child
	n48.nChildren++

	return &n48.nodeHeader
}

// removeChild removes the child with given next byte. If the number of children
// goes below 5 a node4 is returned instead.
func (n *node16) removeChild(txn *Txn, c byte) *nodeHeader {
	idx := n.indexOf(c)
	if idx < 0 {
		// Child doesn't exist
		return &n.nodeHeader
	}

	if n.nChildren > 5 {
		// Remove in place
		removeByteIndex(n.index[0:n.nChildren], idx)
		removeChild(n.children[0:n.nChildren], idx)
		n.nChildren--
		return &n.nodeHeader
	}

	// Convert to a node4
	n4 := txn.newNode4()

	// Copy prefix
	n4.prefixLen = n.prefixLen
	copy(n4.prefix[:], n.prefix[:])

	// Copy children
	for childIdx, childC := range n.index[0:n.nChildren] {
		if childC == c {
			continue
		}
		n4.index[n4.nChildren] = childC
		n4.children[n4.nChildren] = n.children[childIdx]
		n4.nChildren++
	}

	return &n4.nodeHeader
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *node16) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	idx := n.indexOf(c)
	if idx < 0 {
		// Child doesn't exist, don't do anything, this shouldn't really happen...
		return &n.nodeHeader
	}
	n.children[idx] = child
	return &n.nodeHeader
}

// minChild returns the child node with the lowest key or nil if there are no
// children.
func (n *node16) minChild() *nodeHeader {
	if n.nChildren > 0 {
		return n.children[0]
	}
	return nil
}

// maxChild returns the child node with the highest key or nil if there are no
// children.
func (n *node16) maxChild() *nodeHeader {
	if n.nChildren > 0 {
		return n.children[n.nChildren-1]
	}
	return nil
}

// lowerBound returns the child node with the lowest key that is at least as
// large as the search key or nil if there are no keys with a next-byte equal or
// higher than c.
func (n *node16) lowerBound(c byte) *nodeHeader {
	if n.nChildren == 0 {
		return nil
	}
	for i := 0; i < int(n.nChildren); i++ {
		if n.index[i] < c {
			continue
		}
		return n.children[i]
	}
	return nil
}

// upperBound returns the child node with the lowest key that is strictly larger
// than the search key or nil if there are no larger keys.
func (n *node16) upperBound(c byte) *nodeHeader {
	if n.nChildren == 0 {
		return nil
	}
	for i := 0; i < int(n.nChildren); i++ {
		if n.index[i] <= c {
			continue
		}
		return n.children[i]
	}
	return nil
}

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *node16) copy(txn *Txn) *nodeHeader {
	nn := txn.newNode16()
	copyInnerNodeHeader(&nn.innerNodeHeader, &n.innerNodeHeader)
	// Copy index and children
	copy(nn.index[0:16], n.index[0:16])
	copy(nn.children[0:16], n.children[0:16])
	return &nn.nodeHeader
}
