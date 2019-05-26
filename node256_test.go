package art

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func testMakeNode256ChildLeaves(t *testing.T, txn *Txn, n int) []*nodeHeader {
	children := make([]*nodeHeader, 256)
	for i := range children {
		if i == n {
			break
		}
		c := allTheBytes[i]
		k := string([]byte{c, c, c})
		children[c] = testMakeLeaf(txn, k)
	}
	return children
}

func TestNode256FindChild(t *testing.T) {
	txn := &Txn{}

	tests := []struct {
		name     string
		index    []byte
		children []*nodeHeader
		c        byte
		wantKey  string
	}{
		{
			name:     "empty",
			index:    []byte{},
			children: []*nodeHeader{},
			c:        'a',
			wantKey:  "",
		},
		{
			name:     "one-found",
			children: testMakeNode256ChildLeaves(t, txn, 1),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "two-found-0",
			children: testMakeNode256ChildLeaves(t, txn, 2),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "two-found-1",
			children: testMakeNode256ChildLeaves(t, txn, 2),
			c:        'a',
			wantKey:  "aaa",
		},
		{
			name:     "two-not-found",
			children: testMakeNode256ChildLeaves(t, txn, 2),
			c:        '[',
			wantKey:  "",
		},
		{
			name:     "full-found-0",
			children: testMakeNode256ChildLeaves(t, txn, 256),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "full-found-256",
			children: testMakeNode256ChildLeaves(t, txn, 256),
			c:        's',
			wantKey:  "sss",
		},
		{
			name:     "not-found",
			children: testMakeNode256ChildLeaves(t, txn, 250),
			c:        's',
			wantKey:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &node256{
				innerNodeHeader: innerNodeHeader{
					nodeHeader: nodeHeader{
						id:  1,
						typ: typNode256,
					},
					nChildren: uint16(len(tt.children)),
				},
			}
			// Set the ref just so the node is in a consistent state
			n.ref = unsafe.Pointer(n)
			// Need to copy the children since they are not type compatible
			copy(n.children[:], tt.children)
			assertChildHasLeaf(t, &n.nodeHeader, tt.c, tt.wantKey)
		})
	}
}

func TestNode256AddRemoveChild(t *testing.T) {
	txn := &Txn{}
	require := require.New(t)

	// Start with an empty node256
	n := txn.newNode256()
	// Sanity checks
	require.Equal(0, int(n.nChildren))
	require.Equal(typNode256, n.typ)

	var n256h *nodeHeader

	children := testMakeChildLeaves(t, txn, 256)

	// Add children up to 48
	for i, child := range children[:] {
		nh := n.addChild(txn, allTheBytes[i], child)
		// Should not have replaced the node typ
		require.Equal(typNode256, nh.typ)
		gotN := nh.node256()
		require.Exactly(gotN, n)
		// Should have right number of children
		require.Equal(i+1, int(gotN.nChildren))
		// All the children should be found
		for j, wantChild := range children[0 : i+1] {
			assertChildHasLeaf(t, nh, allTheBytes[j], string(wantChild.leafNode().key))
		}
		// Save the node256 for later
		n256h = nh
	}

	// Remove from the node256
	for i := range children[:] {
		nh := n256h.removeChild(txn, allTheBytes[i])
		// Should not have replaced the node typ
		require.Equal(typNode256, nh.typ)
		gotN := nh.node256()
		require.Exactly(gotN, n)
		// Should have right number of children
		require.Equal(256-i-1, int(gotN.nChildren))
		// All the other children should be found
		for j, wantChild := range children[i+1 : 256] {
			assertChildHasLeaf(t, nh, allTheBytes[j+i+1], string(wantChild.leafNode().key))
		}
		if gotN.nChildren == 48+1 {
			// Stop here, next remove should shrink the node
			break
		}
	}

	// Remove the last child
	nh := n256h.removeChild(txn, allTheBytes[255])
	// Should shrink to a node48
	require.Equal(typNode48, nh.typ)
	require.Equal(48, int(nh.node48().nChildren))
	// All the children should be found
	for j, wantChild := range children[256-48 : 256-1] {
		assertChildHasLeaf(t, nh, allTheBytes[256-48+j], string(wantChild.leafNode().key))
	}
}

func TestNode256MinMaxChild(t *testing.T) {
	txn := &Txn{}

	tests := []struct {
		name             string
		children         []*nodeHeader
		wantMin, wantMax string
	}{
		{
			name:     "empty",
			children: []*nodeHeader{},
			wantMin:  "",
			wantMax:  "",
		},
		{
			name:     "full",
			children: testMakeChildLeaves(t, txn, 48),
			wantMin:  "\x00\x00\x00",
			wantMax:  "\xff\xff\xff",
		},
		{
			name:     "one null",
			children: []*nodeHeader{testMakeLeaf(txn, "\x00\x00\x00")},
			wantMin:  "\x00\x00\x00",
			wantMax:  "\x00\x00\x00",
		},
		{
			name:     "one str",
			children: []*nodeHeader{testMakeLeaf(txn, "foo")},
			wantMin:  "foo",
			wantMax:  "foo",
		},
		{
			name: "two str",
			children: []*nodeHeader{
				testMakeLeaf(txn, "foo"),
				testMakeLeaf(txn, "aaa"),
			},
			wantMin: "aaa",
			wantMax: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &txn.newNode256().nodeHeader
			for _, child := range tt.children {
				n = n.addChild(txn, child.leafNode().key[0], child)
			}

			gotMin := n.minChild()
			assertLeafKey(t, gotMin, tt.wantMin)

			gotMax := n.maxChild()
			assertLeafKey(t, gotMax, tt.wantMax)
		})
	}
}

func TestNode256LowerBound(t *testing.T) {
	txn := &Txn{}

	tests := []struct {
		name      string
		children  []*nodeHeader
		key       string
		wantLower string
	}{
		{
			name:      "empty",
			children:  []*nodeHeader{},
			key:       "foo",
			wantLower: "",
		},
		{
			name:      "full, match",
			children:  testMakeChildLeaves(t, txn, 48),
			key:       "aaa",
			wantLower: "aaa",
		},
		{
			name:      "full, no match",
			children:  testMakeChildLeaves(t, txn, 48),
			key:       "aa",
			wantLower: "aaa",
		},
		{
			name:      "full, same lower upper",
			children:  testMakeChildLeaves(t, txn, 48),
			key:       "YYY",
			wantLower: "ZZZ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			n := &txn.newNode256().nodeHeader
			for _, child := range tt.children {
				n = n.addChild(txn, child.leafNode().key[0], child)
			}

			gotLower := n.lowerBound(tt.key[0])
			assertLeafKey(t, gotLower, tt.wantLower)
		})
	}
}
