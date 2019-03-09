package art

// Tree is an immutable radix tree. Write transactions can be performed that
// will return a new tree leaving old nodes untouched. This makes it safe for
// concurrent readers without locks.
type Tree struct {
	root  *nodeHeader
	maxID uint64
	size  int
}

// New returns an empty Tree
func New() *Tree {
	return &Tree{}
}

// Len is used to return the number of elements in the tree
func (t *Tree) Len() int {
	return t.size
}

// Txn starts a new transaction that can be used to mutate the tree
func (t *Tree) Txn() *Txn {
	txn := &Txn{
		maxRootID: t.maxID,
		maxSnapID: t.maxID,
		root:      t.root,
		snap:      t.root,
		size:      t.size,
	}
	return txn
}

// Insert is used to add or update a given key. The return provides
// the new tree, previous value and a bool indicating if any was set.
func (t *Tree) Insert(k []byte, v interface{}) (*Tree, interface{}, bool) {
	txn := t.Txn()
	old, ok := txn.Insert(k, v)
	return txn.Commit(), old, ok
}

// Delete is used to delete a given key. Returns the new tree,
// old value if any, and a bool indicating if the key was set.
func (t *Tree) Delete(k []byte) (*Tree, interface{}, bool) {
	txn := t.Txn()
	old, ok := txn.Delete(k)
	return txn.Commit(), old, ok
}

// DeletePrefix is used to delete all nodes starting with a given prefix. Returns the new tree,
// and a bool indicating if the prefix matched any nodes
func (t *Tree) DeletePrefix(k []byte) (*Tree, bool) {
	txn := t.Txn()
	ok := txn.DeletePrefix(k)
	return txn.Commit(), ok
}

// Root returns the root node of the tree which can be used for richer
// query operations.
func (t *Tree) Root() *APINode {
	return &APINode{
		h: t.root,
	}
}

// Get is used to lookup a specific key, returning
// the value and if it was found
func (t *Tree) Get(k []byte) (interface{}, bool) {
	return nil, false
}
