package art

// node256 is a radix node with >48 children. It has a full 256 byte pointer
// array so lookup is constant time.
type node256 struct {
	h        nodeHeader
	ih       innerNodeHeader
	children [256]*nodeHeader
}

// index returns the child index of the child with the next byte c. If there is
// no such child, -1 is returned.
func (n *node256) indexOf(c byte) int {
	return int(c)
}

// childAt returns the child with the given next byte if any exists or nil.
func (n *node256) childAt(c byte) *nodeHeader {
	return n.children[c]
}

// addChild adds the child to the current node256 in place.
func (n *node256) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	n.children[c] = child
	n.h.nChildren++
	return &n.h
}

// removeChild removes the child with given next byte. If the number of children
// goes below 49 a node48 is returned instead.
func (n *node256) removeChild(txn *Txn, c byte) *nodeHeader {
	idx := n.indexOf(c)
	if idx < 0 {
		// Child doesn't exist
		return &n.h
	}

	if n.h.nChildren > 48 {
		// Remove in place.
		n.children[c] = nil
		n.h.nChildren--
	}

	// Convert to a node48
	n48 := txn.newNode48()

	// Copy prefix
	n48.ih.prefixLen = n.ih.prefixLen
	copy(n48.h.prefix[0:maxPrefixLen], n.h.prefix[0:maxPrefixLen])

	// Copy children
	for childC, child := range n.children {
		if child != nil && childC != int(c) {
			n48.index[childC] = n48.h.nChildren
			n48.children[n48.h.nChildren] = child
			n48.h.nChildren++
		}
	}

	return &n48.h
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *node256) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	n.children[c] = child
	return &n.h
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

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *node256) copy(txn *Txn) *nodeHeader {
	nn := txn.newNode256()
	copyNodeHeader(&nn.h, &n.h)
	// Copy index and children
	copy(nn.children[0:256], n.children[0:256])
	return &nn.h
}
