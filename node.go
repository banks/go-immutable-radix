// Package art implements an immutable Adaptive Radix Tree (ART).
//
// The original paper is https://db.in.tum.de/~leis/papers/ART.pdf. This is
// heavily based on code from github.com/plar/go-adaptive-radix-tree and the C
// github.com/armon/libart.
package art

import (
	"unsafe"
)

const (
	maxPrefixLen = 10

	typLeaf uint8 = iota
	typNode4
	typNode16
	typNode48
	typNode256
)

type nodeHeader struct {
	id  uint64
	ref unsafe.Pointer
	typ uint8
}

type innerNodeHeader struct {
	nodeHeader
	leaf *leafNode
	// prefixLen stores the number of key bytes that are common among all
	// children. It actually has to store O(k) since it might indicate a prefix
	// much longer than the one we can store in maxPrefixLen. 65k is probably
	// enough for a limit on key length though so uint16 should be OK.
	prefixLen uint16
	// nChildren stores the number of children, uint8 is too small to store both 0
	// children and a full node256. Since an inner node never has 0 children we
	// _could_ store number of children -1 but it adds complication for little
	// benefit.
	nChildren uint16
	prefix    [maxPrefixLen]byte
}

type leafNode struct {
	nodeHeader
	key   []byte
	value interface{}
}

// copyInnerNodeHeader copies all the fields from one node header to another except
// for ID and ref which are unique.
func copyInnerNodeHeader(dst, src *innerNodeHeader) {
	// Shallow copy is sufficient because prefix is an embedded array of byte
	// not a slice pointing to a shared array, but we can't just use = since
	// that would override the id and ref in nodeHeader
	dst.leaf = src.leaf
	dst.nChildren = src.nChildren
	dst.prefixLen = src.prefixLen
	dst.prefix = src.prefix
}

func (n *nodeHeader) node4() *node4 {
	return (*node4)(n.ref)
}
func (n *nodeHeader) node16() *node16 {
	return (*node16)(n.ref)
}
func (n *nodeHeader) node48() *node48 {
	return (*node48)(n.ref)
}
func (n *nodeHeader) node256() *node256 {
	return (*node256)(n.ref)
}
func (n *nodeHeader) leafNode() *leafNode {
	return (*leafNode)(n.ref)
}

func (n *nodeHeader) innerLeaf() *leafNode {
	switch n.typ {
	case typLeaf:
		return nil

	case typNode4:
		n4 := n.node4()
		return n4.leaf

	case typNode16:
		n16 := n.node16()
		return n16.leaf

	case typNode48:
		n48 := n.node48()
		return n48.leaf

	case typNode256:
		n256 := n.node256()
		return n256.leaf
	}
	panic("invalid type")
}

func (n *nodeHeader) setInnerLeaf(leaf *leafNode) {
	switch n.typ {
	case typNode4:
		n4 := n.node4()
		n4.leaf = leaf
		return

	case typNode16:
		n16 := n.node16()
		n16.leaf = leaf
		return

	case typNode48:
		n48 := n.node48()
		n48.leaf = leaf
		return

	case typNode256:
		n256 := n.node256()
		n256.leaf = leaf
		return
	}
	panic("invalid type")
}

// prefix returns the effective prefix of a node. If the prefix doesn't fit in
// the prefix array then we find it from the leaf.
func (n *nodeHeader) prefix() []byte {
	pLen, pBytes := n.prefixFields()

	if *pLen <= maxPrefixLen {
		// We have the whole prefix from the node
		return pBytes[0:*pLen]
	}

	// Prefix is too long for node, we have to go find it from the leaf
	minLeaf := n.minChild().leafNode()
	return minLeaf.key[0:*pLen]
}

// prefixFields returns pointers to the prefix len and byte slice if they exist
// for convenience.
func (n *nodeHeader) prefixFields() (*uint16, []byte) {
	switch n.typ {
	case typLeaf:
		// Leaves have no prefix
		return nil, nil
	case typNode4:
		n4 := n.node4()
		return &n4.prefixLen, n4.prefix[:]

	case typNode16:
		n16 := n.node16()
		return &n16.prefixLen, n16.prefix[:]

	case typNode48:
		n48 := n.node48()
		return &n48.prefixLen, n48.prefix[:]

	case typNode256:
		n256 := n.node256()
		return &n256.prefixLen, n256.prefix[:]
	}
	panic("invalid type")
}

func (n *nodeHeader) findChild(c byte) *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().findChild(c)

	case typNode16:
		return n.node16().findChild(c)

	case typNode48:
		return n.node48().findChild(c)

	case typNode256:
		return n.node256().findChild(c)
	}
	panic("invalid type")
}

// addChild adds the child to the current node4 in place if possible or copies
// itself into a node16 and returns that. We assume there is no existing child
// with the same next byte. This MUST be ensured by the caller. Since the caller
// always knows in practice it's cheaper not to check again here.
func (n *nodeHeader) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().addChild(txn, c, child)

	case typNode16:
		return n.node16().addChild(txn, c, child)

	case typNode48:
		return n.node48().addChild(txn, c, child)

	case typNode256:
		return n.node256().addChild(txn, c, child)
	}
	panic("invalid type")
}

// removeChild removes the child with given next byte. Other node types might
// need to shrink and return a new node but node4 never can so always returns
// itself.
func (n *nodeHeader) removeChild(txn *Txn, c byte) *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().removeChild(txn, c)

	case typNode16:
		return n.node16().removeChild(txn, c)

	case typNode48:
		return n.node48().removeChild(txn, c)

	case typNode256:
		return n.node256().removeChild(txn, c)
	}
	panic("invalid type")
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *nodeHeader) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().replaceChild(txn, c, child)

	case typNode16:
		return n.node16().replaceChild(txn, c, child)

	case typNode48:
		return n.node48().replaceChild(txn, c, child)

	case typNode256:
		return n.node256().replaceChild(txn, c, child)
	}
	panic("invalid type")
}

// minChild returns the child node with the lowest key or nil if there are no
// children.
func (n *nodeHeader) minChild() *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().minChild()

	case typNode16:
		return n.node16().minChild()

	case typNode48:
		return n.node48().minChild()

	case typNode256:
		return n.node256().minChild()
	}
	panic("invalid type")
}

// maxChild returns the child node with the highest key or nil if there are no
// children.
func (n *nodeHeader) maxChild() *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().maxChild()

	case typNode16:
		return n.node16().maxChild()

	case typNode48:
		return n.node48().maxChild()

	case typNode256:
		return n.node256().maxChild()
	}
	panic("invalid type")
}

// lowerBound returns the child node with the lowest key that is at least as
// large as the search key or nil if there are no keys with a next-byte equal or
// higher than c.
func (n *nodeHeader) lowerBound(c byte) *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().lowerBound(c)

	case typNode16:
		return n.node16().lowerBound(c)

	case typNode48:
		return n.node48().lowerBound(c)

	case typNode256:
		return n.node256().lowerBound(c)
	}
	panic("invalid type")
}

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *nodeHeader) copy(txn *Txn) *nodeHeader {
	switch n.typ {
	case typLeaf:
		// Leaves have no children
		return nil

	case typNode4:
		return n.node4().copy(txn)

	case typNode16:
		return n.node16().copy(txn)

	case typNode48:
		return n.node48().copy(txn)

	case typNode256:
		return n.node256().copy(txn)
	}
	panic("invalid type")
}

// setPrefix assigned the prefix to the nodeHeader. If the p slice is longer
// than maxPrefixLen then only the first maxPrefixLen bytes will be used.
// Calling this on a non-leaf node will panic.
func (n *nodeHeader) setPrefix(p []byte) {
	pLen, pBytes := n.prefixFields()

	// Write to the byte array and set the length field to the num bytes copied
	*pLen = uint16(copy(pBytes, p))
}

// leftTrimPrefix modifies n in-place by removing l bytes from the prefix.
// Calling this on a non-leaf node will panic.
func (n *nodeHeader) leftTrimPrefix(l uint16) {
	if l < 1 {
		return
	}
	pLen, pBytes := n.prefixFields()
	if l > *pLen {
		l = *pLen
	}
	newLen := *pLen - uint16(l)
	copy(pBytes[0:newLen], pBytes[l:*pLen])
	*pLen = newLen
}

// insertChild inserts a child pointer into a slice of pointers at the specified
// index maintaining current sort order. It does not bounds check and assumes
// that the slice is large enough to accommodate the insertion. The children
// slice should only represent the current size of the children of the node
// although we assume the full child array is allocated underneath so the append
// should never reallocate.
func insertChild(children []*nodeHeader, child *nodeHeader, idx int) {
	// Append to "grow" the slice, should never reallocate so we don't need to
	// return the slice to the caller since the underlying node array has been
	// modified as desired.
	children = append(children, child)
	copy(children[idx+1:], children[idx:])
	children[idx] = child
}

// insertIndex inserts a byte into a node4/16 index.
func insertIndex(index []byte, c byte, idx int) {
	// Append to "grow" the slice, should never reallocate so we don't need to
	// return the slice to the caller since the underlying byte array has been
	// modified as desired.
	index = append(index, c)
	copy(index[idx+1:], index[idx:])
	index[idx] = c
}

// removeChild removes an element from a child array and shuffles any later
// pointers up one to keep it dense.
func removeChild(children []*nodeHeader, idx int) {
	copy(children[idx:], children[idx+1:])
	children[len(children)-1] = nil
}

// removeByteIndex removes an element from a byte slice and shuffles any later
// bytes up one to keep it dense.
func removeByteIndex(bs []byte, idx int) {
	copy(bs[idx:], bs[idx+1:])
	bs[len(bs)-1] = 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minU16(a, b uint16) int {
	if a < b {
		return int(a)
	}
	return int(b)
}

// longestPrefix finds the length of the shared prefix of two strings
func longestPrefix(k1, k2 []byte) int {
	limit := min(len(k1), len(k2))
	for i := 0; i < limit; i++ {
		if k1[i] != k2[i] {
			return i
		}
	}
	return limit
}
