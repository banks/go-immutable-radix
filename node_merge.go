package art

/*
// mergeUpdates recursively merges the passed node as if it contains a tree of
// new values to insert or replace the current tree with. This is used in
// transactions on the immutable radix to merge the update set with the current
// tree. If we end up not changing n, it is returned as-is, but any changes to n
// result in a new node being created to preserve nodes in the committed tree
// being modified when they may be being accessed. updateNode however may be
// re-used as it's transaction local and nothing else is able to access it yet.
// This saves re-allocating nodes used in the transaction's update set in many
// cases.
//
// snapMaxID must be a value greater that or equal to the highest node ID in the
// snapshot being modified (i.e. invariant: n.id <= snapMaxID) and strictly less
// than and node ID in the update tree (invariant: un.id > snapMaxID).
//
// The removed slice if non-nil, will be populated with the IDs of every node in
// the current snapshot that will no longer be in the new snapshot. This allows
// efficient "watching" for changes in bulk based on IDs rather than watching
// arbitrarily many channels.
func (n *nodeHeader) mergeUpdates(snapMaxID uint64, un *nodeHeader,
	removed *[]uint64) *nodeHeader {
	if un == nil {
		return n
	}

	commonPfx := uint8(longestPrefix(n.prefix[0:n.prefixLen], un.prefix[0:un.prefixLen]))

	// Assume we don't have to modify this node. If we do, we replace this with a
	// copy once and mutate the copy.
	newNode := n

	trackRemove := func(id uint64) {
		if removed != nil {
			*removed = append(*removed, id)
		}
	}

	copyNode := func() {
		if newNode.id == n.id {
			newNode = n.copy()
			trackRemove(n.id)
		}
	}

	switch {

	// Both have the same prefix (commonPrefix is the whole of both prefixes).
	// We need to merge the edges recursively.
	case commonPfx == n.prefixLen && commonPfx == un.prefixLen:
		if n.nChildren == 0 {
			// n had no children and same prefix so just use the update (sub)tree
			// directly. We are implicitly removing n from the tree so track that.
			trackRemove(n.id)
			return un
		}
		// Merge recursively
		for i := byte(0); i <= 255; i++ {
			foundInN := n.indexOf(i) > -1
			foundInUN := un.indexOf(i) > -1
			switch {
			case foundInN && foundInUN:
				// Found in both, merge
				copyNode()
				newNode.replaceChild(i,
					newNode.childAt(i).mergeUpdates(snapMaxID, un.childAt(i), removed))

			case foundInN:
				// We have it, un doesn't so no change needed
				continue

			case foundInUN:
				// We don't have it but un does so just use the updated subtree
				copyNode()
				newNode.addChild(i, un.childAt(i))

			default:
				// Neither has it, just carry on the loop!
			}
		}
		// If the other has a leaf and we don't, use their leaf value
		if un.leaf != nil {
			copyNode()
			newNode.leaf = un.leaf
		}
		// If the node ended up with only one child, collapse it with the child if
		// possible.
		if newNode.nChildren == 1 {
			newNode = newNode.mergeWithChild(snapMaxID)
		}
		return newNode

	// The update node is a prefix of this node.
	case commonPfx == un.prefixLen:
		copyNode()
		// We insert n as a child of un with the common prefix removed
		newNode.leftTrimPrefix(commonPfx)
		// Then merge newNode as a child of un
		un.mergeChildUpdates(snapMaxID, newNode.prefix[0], newNode, false, removed)
		if un.nChildren == 1 {
			un = un.mergeWithChild(snapMaxID)
		}
		return un

	// This node is a prefix of the update node
	case commonPfx == n.prefixLen:
		copyNode()
		// We insert the other node as a child, possibly merging it with an existing
		// child we have.
		// Remove the common prefix from the update node.
		un.leftTrimPrefix(commonPfx)
		// Then merge update node as a child of newNode
		newNode.mergeChildUpdates(snapMaxID, un.prefix[0], un, false, removed)
		if newNode.nChildren == 1 {
			newNode = newNode.mergeWithChild(snapMaxID)
		}
		return newNode

	// Both nodes have a non-zero common prefix but neither is a full prefix of
	// the other. Create a new parent with that prefix and insert both children.
	default:
		newParent := newNode4()
		// Give parent the common prefix
		newParent.ih.prefixLen = commonPfx
		copy(newParent.h.prefix[0:commonPfx], n.prefix[0:commonPfx])

		// Copy this node to remove it's prefix
		copyNode()
		newNode.leftTrimPrefix(commonPfx)
		newParent.addChild(newNode.prefix[0], newNode)
		// Add update node with prefix trimmed
		un.leftTrimPrefix(commonPfx)
		newParent.addChild(un.prefix[0], un)
		return n
	}
}

// mergeDeletes is like mergeUpdates but takes a tree of keys that have been
// deleted in a transaction and applies it recursively to the current tree with
// copy-on-write semantics.
func (n *nodeHeader) mergeDeletes(deleteNode *nodeHeader) *nodeHeader {
	return n
}

// mergeWithChild is called if this node ends up having only one child provided
// the merged prefix is less than MaxPrefixLen. By definition a node with only
// one child must be a node4 but the child could be any type so we actually want
// to modify the child to expand it's prefix if possible then replace ourselves
// with it.
func (n *nodeHeader) mergeWithChild(snapMaxID uint64) *nodeHeader {
	// Check if our prefix fits in the child
	child := n.minChild()
	if (child.prefixLen + n.prefixLen) > maxPrefixLen {
		// Can't merge as we don't have space.
		return n
	}

	// We need to update the child's prefix. See if it was created during this
	// transaction already or part of the snapshot.
	if child.id <= snapMaxID {
		// Was part of the snapshot, need to copy it before we can mutate
		child = child.copy()
	}

	newLen := child.prefixLen + n.prefixLen
	// Move current prefix up by the difference
	copy(child.prefix[n.prefixLen:newLen], child.prefix[0:child.prefixLen])
	// Copy parent prefix into child
	copy(child.prefix[0:n.prefixLen], n.prefix[0:n.prefixLen])
	// Update length
	child.prefixLen = newLen
	return child
}

// mergeChild merges the given child with the node own child at the same next
// byte. It assumes n is already mutable.
func (n *nodeHeader) mergeChildUpdates(snapMaxID uint64, c byte, child *nodeHeader, childIsUpdateNode bool, removed *[]uint64) {
	existingChild := n.childAt(c)
	if existingChild == nil {
		// Nothing to merge with, just add it and be done!
		n.addChild(c, child)
		return
	}

	// Need to merge with existing child
	var newChild *nodeHeader
	if childIsUpdateNode {
		newChild = existingChild.mergeUpdates(snapMaxID, child, removed)
	} else {
		// In this case the child being passed is the original snapshot node being
		// merged into the update node (n), We need to reverse the order of merge to
		// make sure leaf updates take precedence as we recurse.
		newChild = child.mergeUpdates(snapMaxID, existingChild, removed)
	}
	n.replaceChild(c, newChild)
}
*/
