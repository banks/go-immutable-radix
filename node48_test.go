package art

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func testNode48Index(t *testing.T, n int) []byte {
	t.Helper()
	index := make([]byte, 256)
	for i, b := range allTheBytes[0:n] {
		index[b] = byte(i + 1)
	}
	return index
}

func TestNode48FindChild(t *testing.T) {
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
			index:    testNode48Index(t, 1),
			children: testMakeChildLeaves(t, txn, 1),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "two-found-0",
			index:    testNode48Index(t, 2),
			children: testMakeChildLeaves(t, txn, 2),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "two-found-1",
			index:    testNode48Index(t, 2),
			children: testMakeChildLeaves(t, txn, 2),
			c:        'a',
			wantKey:  "aaa",
		},
		{
			name:     "two-not-found",
			index:    testNode48Index(t, 2),
			children: testMakeChildLeaves(t, txn, 2),
			c:        '[',
			wantKey:  "",
		},
		{
			name:     "full-found-0",
			index:    testNode48Index(t, 48),
			children: testMakeChildLeaves(t, txn, 48),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "full-found-48",
			index:    testNode48Index(t, 48),
			children: testMakeChildLeaves(t, txn, 48),
			c:        '^',
			wantKey:  "^^^",
		},
		{
			name:     "full-not-found",
			index:    testNode48Index(t, 48),
			children: testMakeChildLeaves(t, txn, 48),
			c:        '(',
			wantKey:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &node48{
				innerNodeHeader: innerNodeHeader{
					nodeHeader: nodeHeader{
						id:  1,
						typ: typNode48,
					},
					nChildren: uint16(len(tt.children)),
				},
			}
			// Set the ref just so the node is in a consistent state
			n.ref = unsafe.Pointer(n)
			// Need to copy the index and children since they are not type compatible
			copy(n.index[:], tt.index)
			copy(n.children[:], tt.children)
			assertChildHasLeaf(t, &n.nodeHeader, tt.c, tt.wantKey)
		})
	}
}

func TestNode48AddRemoveChild(t *testing.T) {
	txn := &Txn{}
	require := require.New(t)

	// Start with an empty node48
	n := txn.newNode48()
	// Sanity checks
	require.Equal(0, int(n.nChildren))
	require.Equal(typNode48, n.typ)

	var n48h *nodeHeader

	children := testMakeChildLeaves(t, txn, 49)

	// Add children up to 48
	for i, child := range children[0:48] {
		nh := n.addChild(txn, allTheBytes[i], child)
		// Should not have replaced the node typ
		require.Equal(typNode48, nh.typ)
		gotN := nh.node48()
		require.Exactly(gotN, n)
		// Should have right number of children
		require.Equal(i+1, int(gotN.nChildren))
		// All the children should be found
		for j, wantChild := range children[0 : i+1] {
			assertChildHasLeaf(t, nh, allTheBytes[j], string(wantChild.leafNode().key))
		}
		// Save the node48 for later
		n48h = nh
	}

	// Add child 49
	nh := n.addChild(txn, allTheBytes[48], children[48])
	// Should grow to a node265
	require.Equal(typNode256, nh.typ)
	require.Equal(49, int(nh.node256().nChildren))
	// All the children should be found
	for j, wantChild := range children {
		assertChildHasLeaf(t, nh, allTheBytes[j], string(wantChild.leafNode().key))
	}

	// Remove from the node48 (remove from node256 tested elsewhere)
	for i := range children[0:48] {
		nh := n48h.removeChild(txn, allTheBytes[i])
		// Should not have replaced the node typ
		require.Equal(typNode48, nh.typ)
		gotN := nh.node48()
		require.Exactly(gotN, n)
		// Should have right number of children
		require.Equal(48-i-1, int(gotN.nChildren))
		// All the other children should be found
		for j, wantChild := range children[i+1 : 48] {
			assertChildHasLeaf(t, nh, allTheBytes[j+i+1], string(wantChild.leafNode().key))
		}
		if gotN.nChildren == 16+1 {
			// Stop here, next remove should shrink the node
			break
		}
	}

	// Remove the last child
	nh = n48h.removeChild(txn, allTheBytes[47])
	// Should shrink to a node16
	require.Equal(typNode16, nh.typ)
	require.Equal(16, int(nh.node16().nChildren))
	// All the children should be found
	for j, wantChild := range children[48-16 : 48-1] {
		assertChildHasLeaf(t, nh, allTheBytes[48-16+j], string(wantChild.leafNode().key))
	}
}

func TestNode48MinMaxChild(t *testing.T) {
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
			n := &txn.newNode48().nodeHeader
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

func TestNode48LowerBound(t *testing.T) {
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

			n := &txn.newNode48().nodeHeader
			for _, child := range tt.children {
				n = n.addChild(txn, child.leafNode().key[0], child)
			}

			gotLower := n.lowerBound(tt.key[0])
			assertLeafKey(t, gotLower, tt.wantLower)
		})
	}
}
