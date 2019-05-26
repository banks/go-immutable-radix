package art

// node4 is a radix node with 0-4 children. It's so small the index doesn't need
// to be sorted for search - we just iterate. We actually do keep it sorted just
// because that makes growing into a node16 simpler and costs very little.
type node4 struct {
	innerNodeHeader
	index    [4]byte
	children [4]*nodeHeader
}

// index returns the child index of the child with the next byte c. If there is
// no such child, -1 is returned.
func (n *node4) indexOf(c byte) int {
	for i := 0; i < int(n.nChildren); i++ {
		if n.index[i] == c {
			return i
		}
	}
	return -1
}

// findChild returns the child with the given next byte if any exists or nil.
func (n *node4) findChild(c byte) *nodeHeader {
	if idx := n.indexOf(c); idx > -1 {
		return n.children[idx]
	}
	return nil
}

// addChild adds the child to the current node4 in place if possible or copies
// itself into a node16 and returns that. We assume there is no existing child
// with the same next byte. This MUST be ensured by the caller. Since the caller
// always knows in practice it's cheaper not to check again here.
func (n *node4) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	if n.nChildren < 4 {
		// Fast path, we have space so update in place
		// Find the right place to insert
		idx := 0
		for i := 0; i < int(n.nChildren); i++ {
			if n.index[i] > c {
				break
			}
			// Current index byte is smaller than the target so target needs to go
			// at least into the next slot
			idx = i + 1
		}
		insertIndex(n.index[0:n.nChildren], c, idx)
		insertChild(n.children[0:n.nChildren], child, idx)
		n.nChildren++
		return &n.nodeHeader
	}

	// Need to grow to a node16
	n16 := txn.newNode16()

	// Copy prefix
	copyInnerNodeHeader(&n16.innerNodeHeader, &n.innerNodeHeader)

	// Copy children
	n16Idx := 0
	inserted := false
	for childIdx, childC := range n.index {
		if !inserted && c < childC {
			// Insert new child before the rest
			n16.index[n16Idx] = c
			n16.children[n16Idx] = child
			n16Idx++
			inserted = true
		}
		// Copy child
		n16.index[n16Idx] = childC
		n16.children[n16Idx] = n.children[childIdx]
		n16Idx++
	}
	n16.nChildren = uint16(n16Idx)

	return &n16.nodeHeader
}

// removeChild removes the child with given next byte. Other node types might
// need to shrink and return a new node but node4 never can so always returns
// itself.
func (n *node4) removeChild(txn *Txn, c byte) *nodeHeader {
	idx := n.indexOf(c)
	if idx < 0 {
		// Child doesn't exist
		return &n.nodeHeader
	}

	// Remove index
	removeByteIndex(n.index[0:n.nChildren], idx)
	removeChild(n.children[0:n.nChildren], idx)
	n.nChildren--

	return &n.nodeHeader
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *node4) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
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
func (n *node4) minChild() *nodeHeader {
	if n.nChildren > 0 {
		return n.children[0]
	}
	return nil
}

// maxChild returns the child node with the highest key or nil if there are no
// children.
func (n *node4) maxChild() *nodeHeader {
	if n.nChildren > 0 {
		return n.children[n.nChildren-1]
	}
	return nil
}

// lowerBound returns the child node with the lowest key that is at least as
// large as the search key or nil if there are no keys with a next-byte equal or
// higher than c.
func (n *node4) lowerBound(c byte) *nodeHeader {
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

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *node4) copy(txn *Txn) *nodeHeader {
	nn := txn.newNode4()
	copyInnerNodeHeader(&nn.innerNodeHeader, &n.innerNodeHeader)
	// Copy index and children
	copy(nn.index[0:4], n.index[0:4])
	copy(nn.children[0:4], n.children[0:4])
	return &nn.nodeHeader
}
