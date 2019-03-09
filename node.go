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
	h         nodeHeader
	nChildren uint8
	prefixLen uint8
	prefix    [maxPrefixLen]byte
}

// copyInnerNodeHeader copies all the fields from one node header to another except
// for ID and ref which are unique.
func copyInnerNodeHeader(dst, src *innerNodeHeader) {
	dst.nChildren = src.nChildren
	dst.prefixLen = src.prefixLen
	copy(dst.prefix[0:src.prefixLen], src.prefix[0:src.prefixLen])
}

type leafNode struct {
	h     nodeHeader
	key   []byte
	value interface{}
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

func (n *innerNodeHeader) indexOf(c byte) int {
	switch {
	case n.nChildren < 5:
		return n.node4().indexOf(c)

	case n.nChildren < 17:
		return n.node16().indexOf(c)

	case n.nChildren < 49:
		return n.node48().indexOf(c)

	default:
		return n.node256().indexOf(c)
	}
}

// childAt returns the child with the given next byte if any exists or nil.
func (n *innerNodeHeader) childAt(c byte) *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().childAt(c)

	case n.nChildren < 17:
		return n.node16().childAt(c)

	case n.nChildren < 49:
		return n.node48().childAt(c)

	default:
		return n.node256().childAt(c)
	}
}

// addChild adds the child to the current node4 in place if possible or copies
// itself into a node16 and returns that. We assume there is no existing child
// with the same next byte. This MUST be ensured by the caller. Since the caller
// always knows in practice it's cheaper not to check again here.
func (n *innerNodeHeader) addChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().addChild(txn, c, child)

	case n.nChildren < 17:
		return n.node16().addChild(txn, c, child)

	case n.nChildren < 49:
		return n.node48().addChild(txn, c, child)

	default:
		return n.node256().addChild(txn, c, child)
	}
}

// removeChild removes the child with given next byte. Other node types might
// need to shrink and return a new node but node4 never can so always returns
// itself.
func (n *innerNodeHeader) removeChild(txn *Txn, c byte) *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().removeChild(txn, c)

	case n.nChildren < 17:
		return n.node16().removeChild(txn, c)

	case n.nChildren < 49:
		return n.node48().removeChild(txn, c)

	default:
		return n.node256().removeChild(txn, c)
	}
}

// replaceChild replaces a child with a new node. It assumes the child is known
// to exist and is a no-op if it doesn't.
func (n *innerNodeHeader) replaceChild(txn *Txn, c byte, child *nodeHeader) *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().replaceChild(txn, c, child)

	case n.nChildren < 17:
		return n.node16().replaceChild(txn, c, child)

	case n.nChildren < 49:
		return n.node48().replaceChild(txn, c, child)

	default:
		return n.node256().replaceChild(txn, c, child)
	}
}

// minChild returns the child node with the lowest key or nil if there are no
// children.
func (n *innerNodeHeader) minChild() *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().minChild()

	case n.nChildren < 17:
		return n.node16().minChild()

	case n.nChildren < 49:
		return n.node48().minChild()

	default:
		return n.node256().minChild()
	}
}

// maxChild returns the child node with the highest key or nil if there are no
// children.
func (n *innerNodeHeader) maxChild() *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().maxChild()

	case n.nChildren < 17:
		return n.node16().maxChild()

	case n.nChildren < 49:
		return n.node48().maxChild()

	default:
		return n.node256().maxChild()
	}
}

// copy returns a new copy of the current node with the same contents but a new
// ID.
func (n *innerNodeHeader) copy(txn *Txn) *nodeHeader {
	switch {
	case n.nChildren < 5:
		return n.node4().copy(txn)

	case n.nChildren < 17:
		return n.node16().copy(txn)

	case n.nChildren < 49:
		return n.node48().copy(txn)

	default:
		return n.node256().copy(txn)
	}
}

// setPrefix assigned the prefix to the nodeHeader. If the p slice is longer
// than maxPrefixLen then only the first maxPrefixLen bytes will be used.
func (n *innerNodeHeader) setPrefix(p []byte) {
	n.prefixLen = uint8(copy(n.prefix[0:maxPrefixLen], p))
}

// leftTrimPrefix modifies n in-place by removing l bytes from the prefix
func (n *innerNodeHeader) leftTrimPrefix(l uint8) {
	if l < 1 {
		return
	}
	if l > n.prefixLen {
		l = n.prefixLen
	}
	newLen := n.prefixLen - uint8(l)
	copy(n.prefix[0:newLen], n.prefix[l:n.prefixLen])
	n.prefixLen = newLen
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

// longestPrefix finds the length of the shared prefix
// of two strings
func longestPrefix(k1, k2 []byte) int {
	max := len(k1)
	if l := len(k2); l < max {
		max = l
	}
	i := 0
	for ; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}
