package art

// node256 is a radix node with >48 children. It has a full 256 byte pointer
// array so lookup is constant time.
type node256 struct {
	innerNodeHeader
	children [256]*nodeHeader
}

// index returns the child index of the child with the next byte c. If there is
// no such child, -1 is returned.
func (n *node256) indexOf(c byte) int {
	return int(c)
}

// findChild returns the child with the given next byte if any exists or nil.
func (n *node256) findChild(c byte) *nodeHeader {
	return n.children[c]
}

// addChild adds the child to the current node256 in place.
func (n *node256) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	n.children[c] = child
	n.nChildren++
	return &n.nodeHeader
}

// removeChild removes the child with given next byte. If the number of children
// goes below 49 a node48 is returned instead.
func (n *node256) removeChild(txn *Txn, c byte) *nodeHeader {
	idx := n.indexOf(c)
	if idx < 0 {
		// Child doesn't exist
		return &n.nodeHeader
	}

	if n.nChildren > 48 {
		// Remove in place.
		n.children[c] = nil
		n.nChildren--
		return &n.nodeHeader
	}

	// Convert to a node48
	n48 := txn.newNode48()

	// Copy prefix
	n48.prefixLen = n.prefixLen
	copy(n48.prefix[:], n.prefix[:])

	// Copy children
	for childC, child := range n.children {
		if child != nil && childC != int(c) {
			n48.index[childC] = byte(n48.nChildren + 1)
			n48.children[n48.nChildren] = child
			n48.nChildren++
		}
	}

	return &n48.nodeHeader
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *node256) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	n.children[c] = child
	return &n.nodeHeader
}

// minChild returns the child node with the lowest key or nil if there are no
// children.
func (n *node256) minChild() *nodeHeader {
	// Find first byte index at which a child exists
	for i := 0; i <= 255; i++ {
		if n.children[i] != nil {
			return n.children[i]
		}
	}
	return nil
}

// maxChild returns the child node with the highest key or nil if there are no
// children.
func (n *node256) maxChild() *nodeHeader {
	// Find last byte index at which a child exists
	for i := 255; i >= 0; i-- {
		if n.children[i] != nil {
			return n.children[i]
		}
	}
	return nil
}

// lowerBound returns the child node with the lowest key that is at least as
// large as the search key or nil if there are no keys with a next-byte equal or
// higher than c.
func (n *node256) lowerBound(c byte) *nodeHeader {
	if n.nChildren == 0 {
		return nil
	}
	return nil
}

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *node256) copy(txn *Txn) *nodeHeader {
	nn := txn.newNode256()
	copyInnerNodeHeader(&nn.innerNodeHeader, &n.innerNodeHeader)
	// Copy index and children
	copy(nn.children[0:256], n.children[0:256])
	return &nn.nodeHeader
}
