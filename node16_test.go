package art

import (
	"bytes"
	"sort"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

// allTheBytes is a random permutation of all single byte values The first four
// are selected to exercise some edge cases in ordering and byte value handling
// for all node sizes.
var allTheBytes = []byte{'Z', 'a', 0x0, 0xff, '1', '-', '}', '_', '#', '~', ')', 0x81, 0xe5, '0', 0x6, '^', 'E', 0x14, 0xc2, 0xec, 'O', 0x9c, 'C', 'd', 0xef, 0x98, 0x95, ']', '[', '8', 0x8, 0xb7, '*', 0x94, 'r', ';', '9', 0x5, 'p', 0x97, '6', 0x4, 'm', 0x91, 0xe4, 0xc4, 0xfa, 'h', 0xa9, 'k', 'V', '@', 'b', 0xb5, 0xc8, ':', 0xc7, 'F', 0x8a, 0xb3, '<', 0xf9, '"', '{', 0x1e, 0x16, '|', 0xf0, 0xc9, 0x84, 0xda, 0x15, 'J', 'S', '\'', 0xdf, 'I', 'X', 0x88, 0x1b, 0x85, 0xa, 'Y', '3', 0xd7, 0xfb, 'l', 0x3, 0xeb, 0xf1, 0x13, 'f', 'G', '&', 0xa6, 0xdc, 'n', 0x17, 0xe8, 0x19, 0xac, 0xd2, 0x8e, 0xd3, 'y', 0xf2, 'K', 0xd0, 0xc3, 0xcb, 0xe2, 0xfd, 0xb0, 0x11, 'B', 0x9e, 0xe7, 0xed, 'c', 0xfe, 0xad, 0xdd, 'u', 0x8b, 0xd5, 0x82, 'U', 0xb2, 0xbb, 'T', '\\', ',', 0xa4, 0xf7, 'z', ' ', 0x7f, 0xb1, 0xaa, 0x9b, 'o', 0xb9, 0xab, '=', 'L', 0xb8, 0xea, 0xc0, 0x10, 'j', 0xa0, 0xcc, 0x99, 0xa1, 0xba, 0x83, 0x1c, 0x89, '%', 0xd8, 0xf8, '7', 'H', '2', 0x1a, '.', '5', 0xe0, 0x7, 0xd9, 0xbd, 'x', 0xdb, 0xa7, 'w', 0xb, 0xfc, 'A', 0x87, '`', 0xde, 'D', 0x90, 0xd6, 0xe3, 'e', 0xcf, 'g', 0xd4, 0xaf, 0x9d, 0x8d, 0xa8, 'R', 0xa3, '/', '4', 0xf, 'q', 0xe6, 0xf5, 't', '+', 'P', 0xf6, '!', 0xc6, 0xc5, 0x92, 0xc1, 0xd, 0x1f, 0x18, 0x8f, 0xc, 0x12, 'v', 0xe, '>', 0x9a, 'N', 'Q', 0x86, 0xa2, 'i', '?', 0xf4, 'M', 0xbe, 0xd1, 0x96, 0xe9, 0x9f, 0xca, 0xbf, '(', 'W', 0xb4, 0xbc, '$', 0xee, 0x9, 0x8c, 0x80, 0x93, 0xae, 0x1, 0x2, 0xb6, 0xf3, 0x1d, 's', 0xcd, 0xe1, 0xa5, 0xce}

func testMakeChildLeaves(t *testing.T, txn *Txn, n int) []*nodeHeader {
	children := make([]*nodeHeader, n)
	for i := range children {
		c := allTheBytes[i]
		k := string([]byte{c, c, c})
		children[i] = testMakeLeaf(txn, k)
	}
	return children
}

func testSortBytes(bs []byte) []byte {
	sorted := make([]byte, len(bs))
	copy(sorted, bs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return sorted
}

func testSortChildren(children []*nodeHeader) {
	sort.Slice(children, func(i, j int) bool {
		return bytes.Compare(children[i].leafNode().key, children[j].leafNode().key) < 0
	})
}

func TestNode16FindChild(t *testing.T) {
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
			index:    allTheBytes[0:1],
			children: testMakeChildLeaves(t, txn, 1),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "two-found-0",
			index:    allTheBytes[0:2],
			children: testMakeChildLeaves(t, txn, 2),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "two-found-1",
			index:    allTheBytes[0:2],
			children: testMakeChildLeaves(t, txn, 2),
			c:        'a',
			wantKey:  "aaa",
		},
		{
			name:     "two-not-found",
			index:    allTheBytes[0:2],
			children: testMakeChildLeaves(t, txn, 2),
			c:        '[',
			wantKey:  "",
		},
		{
			name:     "full-found-0",
			index:    allTheBytes[0:16],
			children: testMakeChildLeaves(t, txn, 16),
			c:        'Z',
			wantKey:  "ZZZ",
		},
		{
			name:     "full-found-16",
			index:    allTheBytes[0:16],
			children: testMakeChildLeaves(t, txn, 16),
			c:        '^',
			wantKey:  "^^^",
		},
		{
			name:     "full-not-found",
			index:    allTheBytes[0:16],
			children: testMakeChildLeaves(t, txn, 16),
			c:        '[',
			wantKey:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &node16{
				innerNodeHeader: innerNodeHeader{
					nodeHeader: nodeHeader{
						id:  1,
						typ: typNode16,
					},
					nChildren: uint16(len(tt.children)),
				},
			}
			// Set the ref just so the node is in a consistent state
			n.ref = unsafe.Pointer(n)
			// Need to copy the index and children since they are not type compatible
			copy(n.index[:], testSortBytes(tt.index))
			testSortChildren(tt.children)
			copy(n.children[:], tt.children)
			assertChildHasLeaf(t, &n.nodeHeader, tt.c, tt.wantKey)
		})
	}
}

func TestNode16AddRemoveChild(t *testing.T) {
	txn := &Txn{}
	require := require.New(t)

	// Start with an empty node16
	n := txn.newNode16()
	// Sanity checks
	require.Equal(0, int(n.nChildren))
	require.Equal(typNode16, n.typ)

	var n16h *nodeHeader

	children := testMakeChildLeaves(t, txn, 17)

	// Add children up to 16
	for i, child := range children[0:16] {
		nh := n.addChild(txn, allTheBytes[i], child)
		// Should not have replaced the node typ
		require.Equal(typNode16, nh.typ)
		gotN := nh.node16()
		require.Exactly(gotN, n)
		// Should have right number of children
		require.Equal(i+1, int(gotN.nChildren))
		// All the children should be found
		for j, wantChild := range children[0 : i+1] {
			assertChildHasLeaf(t, nh, allTheBytes[j], string(wantChild.leafNode().key))
		}
		// Save the node16 for later
		n16h = nh
	}

	// Add child 17
	nh := n.addChild(txn, allTheBytes[16], children[16])
	// Should grow to a node48
	require.Equal(typNode48, nh.typ)
	require.Equal(17, int(nh.node48().nChildren))
	// All the children should be found
	for j, wantChild := range children {
		assertChildHasLeaf(t, nh, allTheBytes[j], string(wantChild.leafNode().key))
	}

	// Remove from the node16 (remove from node48 tested elsewhere)
	for i := range children[0:16] {
		nh := n16h.removeChild(txn, allTheBytes[i])
		// Should not have replaced the node typ
		require.Equal(typNode16, nh.typ)
		gotN := nh.node16()
		require.Exactly(gotN, n)
		// Should have right number of children
		require.Equal(15-i, int(gotN.nChildren))
		// All the other children should be found
		for j, wantChild := range children[i+1 : 16] {
			assertChildHasLeaf(t, nh, allTheBytes[j+i+1], string(wantChild.leafNode().key))
		}
		if gotN.nChildren == 5 {
			// Stop here, next remove should shrink the node
			break
		}
	}

	// Remove the last child
	nh = n16h.removeChild(txn, allTheBytes[15])
	// Should shrink to a node4
	require.Equal(typNode4, nh.typ)
	require.Equal(4, int(nh.node4().nChildren))
	// All the children should be found
	for j, wantChild := range children[11:15] {
		assertChildHasLeaf(t, nh, allTheBytes[11+j], string(wantChild.leafNode().key))
	}
}

func TestNode16MinMaxChild(t *testing.T) {
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
			children: testMakeChildLeaves(t, txn, 16),
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
			n := &txn.newNode16().nodeHeader
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

func TestNode16LowerBound(t *testing.T) {
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
			children:  testMakeChildLeaves(t, txn, 16),
			key:       "aaa",
			wantLower: "aaa",
		},
		{
			name:      "full, no match",
			children:  testMakeChildLeaves(t, txn, 16),
			key:       "aa",
			wantLower: "aaa",
		},
		{
			name:      "full, same lower upper",
			children:  testMakeChildLeaves(t, txn, 16),
			key:       "YYY",
			wantLower: "ZZZ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			n := &txn.newNode16().nodeHeader
			for _, child := range tt.children {
				n = n.addChild(txn, child.leafNode().key[0], child)
			}

			gotLower := n.lowerBound(tt.key[0])
			assertLeafKey(t, gotLower, tt.wantLower)
		})
	}
}
