package art

import (
	"bytes"
	"unsafe"

	"github.com/y0ssar1an/q"
)

// Txn is a transaction on the tree. This transaction is applied
// atomically and returns a new tree when committed. A transaction
// is not thread safe, and should only be used by a single goroutine.
type Txn struct {
	maxRootID uint64
	root      *nodeHeader
	maxSnapID uint64
	snap      *nodeHeader
	size      int

	// trackMutate enables chan-based mutation watching for this transaction.
	trackMutate bool

	mutateSet map[uint64]struct{}
}

// TrackMutate can be used to toggle if mutations are tracked using channels. If
// this is enabled then notifications will be issued for affected internal nodes
// and leaves when the transaction is committed.
// TODO(banks) note the more efficient mechanism here when implemented.
func (t *Txn) TrackMutate(track bool) {
	t.trackMutate = track
}

// Insert is used to add or update a given key. The return provides
// the previous value and a bool indicating if any was set.
func (t *Txn) Insert(k []byte, v interface{}) (interface{}, bool) {
	newRoot, oldVal, replaced := t.insert(t.root, k, v, 0)
	t.root = newRoot
	return oldVal, replaced
}

// insert performs a recursive insertion, copying nodes if they are from the
// original snapshot
func (t *Txn) insert(n *nodeHeader, k []byte, v interface{}, offset int) (*nodeHeader, interface{}, bool) {
	if n == nil {
		// Replace with a leaf
		newLeaf := t.newLeafNode(k, v)
		return &newLeaf.h, nil, false
	}

	// Is this a leaf node?
	if n.nChildren == 0 {
		leaf := n.leafNode()

		// Is the key identical? Replace value
		if bytes.Equal(leaf.key, k) {
			// Replace leaf
			newLeaf := t.newLeafNode(k, v)
			t.discard(n.id)
			return &newLeaf.h, leaf.value, true
		}

		// New value split leaf into node4
		splitNode := t.newNode4()

		// Find the longest common prefix between the existing and new leaf
		commonPrefixLen := longestPrefix(leaf.key[offset:], k[offset:])

		splitNode.h.setPrefix(k[offset : offset+commonPrefixLen])

		q.Q(offset, commonPrefixLen, k, leaf.key)

		// Leafs don't bother storing prefix since they have the whole key anyway
		// so we can re-use the same leaf node without a copy.
		nextByte := uint8(0)
		if offset+commonPrefixLen < len(leaf.key) {
			nextByte = leaf.key[offset+commonPrefixLen]
		}
		splitNode.addChild(t, nextByte, n)

		// Create new leaf
		newLeaf := t.newLeafNode(k, v)
		// If the key being inserted is a prefix of the current key, we have not
		// more bytes to use as the pivot. Most other ART implementations just crash
		// or break in this case or require null-terminated keys. Instead we
		// re-purpose the null index byte for the leaf BUT we flag that with the
		// hasLeaf bool which means we can still correctly handle keys with null
		// bytes.
		nextByte = uint8(0)
		if offset+commonPrefixLen < len(k) {
			nextByte = k[offset+commonPrefixLen]
		}
		if nextByte == 0 {
			splitNode.h.nullByteIsLeaf = true
		}
		splitNode.addChild(t, nextByte, &newLeaf.h)
		return &splitNode.h, nil, false
	}

	if n.prefixLen > 0 {
		lcp := longestPrefix(k, n.prefix[0:n.prefixLen])
		if lcp > int(n.prefixLen) {
			// Our prefix is a a prefix of the common prefix! So consume the length
			// and continue recursing!
			offset += int(n.prefixLen)
			goto RECURSE
		}

		// Need to create a new split node with the common prefix
		splitNode := t.newNode4()
		splitNode.h.setPrefix(k[offset : offset+lcp])

		// Copy ourselves since we need to truncate the prefix
		newNode := t.copyIfNeeded(n)
		newNode.leftTrimPrefix(uint8(lcp))
		splitNode.addChild(t, n.prefix[lcp], newNode)

		// Create a new leaf
		newLeaf := t.newLeafNode(k, v)
		splitNode.addChild(t, k[lcp], &newLeaf.h)
		return &splitNode.h, nil, false
	}

RECURSE:

	// Find the next node to recurse to
	child := n.childAt(k[offset])
	if child != nil {
		return t.insert(child, k, v, offset+1)
	}

	// No child just insert a new leaf
	newLeaf := t.newLeafNode(k, v)
	newNode := t.copyIfNeeded(n)
	newNode.addChild(t, k[offset], &newLeaf.h)
	t.discard(n.id)
	return newNode, nil, false
}

func (t *Txn) copyIfNeeded(n *nodeHeader) *nodeHeader {
	if n.id <= t.maxSnapID {
		// The old node will no longer be in the tree
		t.discard(n.id)
		return n.copy(t)
	}
	return n
}

func (t *Txn) discard(id uint64) {
	// Ignore nodes that were never in the snapshot before the txn.
	if id > t.maxSnapID {
		return
	}
	if t.mutateSet == nil {
		t.mutateSet = make(map[uint64]struct{})
	}
	t.mutateSet[id] = struct{}{}
}

// Delete is used to delete a given key. Returns the old value if any,
// and a bool indicating if the key was set.
func (t *Txn) Delete(k []byte) (interface{}, bool) {
	return nil, false
}

// DeletePrefix is used to delete an entire subtree that matches the prefix
// This will delete all nodes under that prefix
func (t *Txn) DeletePrefix(prefix []byte) bool {
	return false
}

func (t *Txn) Commit() *Tree {
	t.Notify()
	return t.CommitOnly()
}

func (t *Txn) CommitOnly() *Tree {
	return &Tree{
		root:  t.root,
		maxID: t.maxRootID,
		size:  t.size,
	}
}

func (t *Txn) Get(k []byte) (interface{}, bool) {
	return nil, false
}

func (t *Txn) GetWatch(k []byte) (<-chan struct{}, interface{}, bool) {
	return nil, nil, false
}

func (t *Txn) Notify() {

}

// Root returns the current root of the radix tree within this
// transaction. The root is not safe across insert and delete operations,
// but can be used to read the current state during a transaction.
func (t *Txn) Root() *APINode {
	return &APINode{t.root}
}

func (t *Txn) nextID() uint64 {
	t.maxRootID++
	return t.maxRootID
}

func (t *Txn) newNode4() *node4 {
	n := &node4{h: nodeHeader{id: t.nextID()}}
	n.h.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newNode16() *node16 {
	n := &node16{h: nodeHeader{id: t.nextID()}}
	n.h.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newNode48() *node48 {
	n := &node48{h: nodeHeader{id: t.nextID()}}
	n.h.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newNode256() *node256 {
	n := &node256{h: nodeHeader{id: t.nextID()}}
	n.h.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newLeafNode(k []byte, v interface{}) *leafNode {
	n := &leafNode{h: nodeHeader{id: t.nextID()}, key: k, value: v}
	n.h.ref = unsafe.Pointer(n)
	return n
}
