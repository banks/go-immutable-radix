package art

import (
	"bytes"
	"unsafe"
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
		return &newLeaf.nodeHeader, nil, false
	}

	// Is this a leaf node?
	if n.typ == typLeaf {
		leaf := n.leafNode()

		// Is the key identical? Replace value
		if bytes.Equal(leaf.key, k) {
			// Replace leaf
			newLeaf := t.newLeafNode(k, v)
			t.discard(n.id)
			return &newLeaf.nodeHeader, leaf.value, true
		}

		// New value split leaf into node4
		splitNode := &t.newNode4().nodeHeader

		// Find the longest common prefix between the existing and new leaf
		commonPrefixLen := longestPrefix(leaf.key[offset:], k[offset:])

		splitNode.setPrefix(k[offset : offset+commonPrefixLen])

		// If the key being inserted is a prefix of the current leaf key, we have no
		// more bytes to use as the pivot. Most other ART implementations just crash
		// or break in this case or require null-terminated keys and no other null
		// bytes. Instead we store leaves directly in inner nodes too.
		if offset+commonPrefixLen == len(leaf.key) {
			// Existing Leaf is prefix of the key being inserted. Insert existing leaf
			// as an inner node. Leafs don't bother storing prefix since they have the
			// whole key anyway so we can re-use the same leaf node without a copy.
			splitNode.setInnerLeaf(leaf)
		} else {
			// Otherwise insert the existing leaf as a child
			splitNode = splitNode.addChild(t, leaf.key[offset+commonPrefixLen], n)
		}

		// Create new leaf
		newLeaf := t.newLeafNode(k, v)
		if offset+commonPrefixLen == len(k) {
			splitNode.setInnerLeaf(newLeaf)
		} else {
			// Otherwise insert the new leaf as a child
			splitNode = splitNode.addChild(t, k[offset+commonPrefixLen], &newLeaf.nodeHeader)
		}
		// No discard since we re-used the existing leaf node in any case above
		return splitNode, nil, false
	}

	if offset >= len(k) {
		// We've already exhausted the key's bytes which means it belongs as a leaf
		// at this inner node level.
		newLeaf := t.newLeafNode(k, v)
		newNode := t.copyIfNeeded(n)
		newNode.setInnerLeaf(newLeaf)
		t.discard(n.id)
		if oldLeaf := n.innerLeaf(); oldLeaf != nil {
			// There was a leaf in this inner node before, discard that too and return
			// it's old value.
			t.discard(oldLeaf.id)
			return newNode, oldLeaf.value, true
		}
		return newNode, nil, false
	}

	prefix := n.prefix()
	if len(prefix) > 0 {
		lcp := longestPrefix(k[offset:], prefix)
		if lcp >= len(prefix) {
			// Our prefix is a a prefix of the common prefix! So consume the
			// length and continue recursing!
			offset += len(prefix)
			goto RECURSE
		}

		// Need to create a new split node with the common prefix
		splitNode := &t.newNode4().nodeHeader
		splitNode.setPrefix(k[offset : offset+lcp])

		// Copy ourselves since we need to truncate the prefix
		newNode := t.copyIfNeeded(n)
		newNode.leftTrimPrefix(uint16(lcp))
		splitNode = splitNode.addChild(t, prefix[lcp], newNode)

		// Create a new leaf
		newLeaf := t.newLeafNode(k, v)
		splitNode = splitNode.addChild(t, k[lcp], &newLeaf.nodeHeader)
		return splitNode, nil, false
	}

RECURSE:

	// Find the next node to recurse to
	child := n.findChild(k[offset])
	if child != nil {
		newChild, old, existed := t.insert(child, k, v, offset+1)
		// Copy node to change child pointer
		newNode := t.copyIfNeeded(n)
		newNode = newNode.replaceChild(t, k[offset], newChild)
		// Don't discard child as it already discarded itself if necessary
		return newNode, old, existed
	}

	// No child just insert a new leaf
	newLeaf := t.newLeafNode(k, v)
	newNode := t.copyIfNeeded(n)
	newNode = newNode.addChild(t, k[offset], &newLeaf.nodeHeader)
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
	n := &node4{
		innerNodeHeader: innerNodeHeader{
			nodeHeader: nodeHeader{
				id:  t.nextID(),
				typ: typNode4,
			},
		},
	}
	n.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newNode16() *node16 {
	n := &node16{
		innerNodeHeader: innerNodeHeader{
			nodeHeader: nodeHeader{
				id:  t.nextID(),
				typ: typNode16,
			},
		},
	}
	n.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newNode48() *node48 {
	n := &node48{
		innerNodeHeader: innerNodeHeader{
			nodeHeader: nodeHeader{
				id:  t.nextID(),
				typ: typNode48,
			},
		},
	}
	n.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newNode256() *node256 {
	n := &node256{
		innerNodeHeader: innerNodeHeader{
			nodeHeader: nodeHeader{
				id:  t.nextID(),
				typ: typNode256,
			},
		},
	}
	n.ref = unsafe.Pointer(n)
	return n
}

func (t *Txn) newLeafNode(k []byte, v interface{}) *leafNode {
	n := &leafNode{
		nodeHeader: nodeHeader{
			id:  t.nextID(),
			typ: typLeaf,
		},
		key:   k,
		value: v,
	}
	n.ref = unsafe.Pointer(n)
	return n
}
