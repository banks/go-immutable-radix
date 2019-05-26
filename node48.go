package art

// node48 is a radix node with 17-48 children. It has a full 256 byte index so
// lookup is constant time. As in the original paper we use the full 8 bits for
// each entry for lookup speed over saving the extra 2 bytes per entry since
// only 48 indexes are needed.
type node48 struct {
	innerNodeHeader
	index    [256]byte
	children [48]*nodeHeader
}

// index returns the child index of the child with the next byte c. If there is
// no such child, -1 is returned.
func (n *node48) indexOf(c byte) int {
	return int(n.index[c]) - 1
}

// findChild returns the child with the given next byte if any exists or nil.
func (n *node48) findChild(c byte) *nodeHeader {
	if idx := n.indexOf(c); idx > -1 {
		return n.children[idx]
	}
	return nil
}

// addChild adds the child to the current node48 in place if possible or copies
// itself into a node48 and returns that. We assume there is no existing child
// with the same next byte. This MUST be ensured by the caller. Since the caller
// always knows in practice it's cheaper not to check again here.
func (n *node48) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	if n.nChildren < 48 {
		// Fast path, we have space so update in place. No need to keep children
		// sorted since the index stores the offset in O(1) lookup.
		idx := n.nChildren
		n.index[c] = byte(idx + 1) // +1 because 0 is "no child"
		n.children[idx] = child
		n.nChildren++
		return &n.nodeHeader
	}

	// Need to grow to a node256
	n256 := txn.newNode256()

	// Copy prefix
	n256.prefixLen = n.prefixLen
	copy(n256.prefix[:], n.prefix[:])

	// Copy children
	for childC, offset := range n.index {
		if offset > 0 {
			n256.children[childC] = n.children[offset-1]
			n256.nChildren++
		}
	}
	// Add new child
	n256.children[c] = child
	n256.nChildren++

	return &n256.nodeHeader
}

// removeChild removes the child with given next byte. If the number of children
// goes below 17 a node16 is returned instead.
func (n *node48) removeChild(txn *Txn, c byte) *nodeHeader {
	idx := n.indexOf(c)
	if idx < 0 {
		// Child doesn't exist
		return &n.nodeHeader
	}

	if n.nChildren > (16 + 1) {
		// Remove in place. First rewrite the index to remove the edge and shuffle
		// all children with higher offset down one.
		oldIdx := n.index[c]
		n.index[c] = 0
		// Note the int cast is needed - if i is a byte then the last i++ wraps
		// around to 0 and makes it an infinite loop.
		for i := int(0); i < 256; i++ {
			// Only move if it was after the removed child in the array
			if n.index[i] > oldIdx {
				n.index[i]--
			}
		}
		removeChild(n.children[0:n.nChildren], idx)
		n.nChildren--
		return &n.nodeHeader
	}

	// Convert to a node16
	n16 := txn.newNode16()

	// Copy prefix
	n16.prefixLen = n.prefixLen
	copy(n16.prefix[:], n.prefix[:])

	// Copy children
	for childC, offset := range n.index {
		if offset > 0 && byte(childC) != c {
			n16.children[childC] = n.children[offset-1]
			n16.nChildren++
		}
	}

	return &n16.nodeHeader
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *node48) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
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
func (n *node48) minChild() *nodeHeader {
	// Find first byte index at which a child exists
	for i := 0; i <= 255; i++ {
		if n.index[i] > 0 {
			return n.children[n.index[i]-1]
		}
	}
	return nil
}

// maxChild returns the child node with the highest key or nil if there are no
// children.
func (n *node48) maxChild() *nodeHeader {
	// Find last byte index at which a child exists
	for i := 255; i >= 0; i-- {
		if n.index[i] > 0 {
			return n.children[n.index[i]-1]
		}
	}
	return nil
}

// lowerBound returns the child node with the lowest key that is at least as
// large as the search key or nil if there are no keys with a next-byte equal or
// higher than c.
func (n *node48) lowerBound(c byte) *nodeHeader {
	if n.nChildren == 0 {
		return nil
	}
	// Iterate from the key offset until we find a child
	for _, offset := range n.index[c:] {
		if offset > 0 {
			return n.children[offset-1]
		}
	}
	return nil
}

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *node48) copy(txn *Txn) *nodeHeader {
	nn := txn.newNode48()
	copyInnerNodeHeader(&nn.innerNodeHeader, &n.innerNodeHeader)
	// Copy index and children
	copy(nn.index[0:256], n.index[0:256])
	copy(nn.children[0:48], n.children[0:48])
	return &nn.nodeHeader
}
