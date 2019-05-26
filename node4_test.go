package art

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func testMakeLeaf(txn *Txn, key string) *nodeHeader {
	l := txn.newLeafNode([]byte(key), key)
	return &l.nodeHeader
}

func assertChildHasLeaf(t *testing.T, n *nodeHeader, c byte, key string) {
	t.Helper()
	child := n.findChild(c)
	assertLeafKey(t, child, key)
}

func assertLeafKey(t *testing.T, n *nodeHeader, key string) {
	t.Helper()
	if key == "" {
		require.Nil(t, n)
	} else {
		require.NotNilf(t, n, "for key %q", key)
		require.Equal(t, typLeaf, n.typ)
		leaf := n.leafNode()
		require.Equal(t, key, string(leaf.key))
	}
}

func TestNode4FindChild(t *testing.T) {
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
			index:    []byte{'f'},
			children: []*nodeHeader{testMakeLeaf(txn, "foo")},
			c:        'f',
			wantKey:  "foo",
		},
		{
			name:     "two-found-0",
			index:    []byte{'b', 'f'},
			children: []*nodeHeader{testMakeLeaf(txn, "bar"), testMakeLeaf(txn, "foo")},
			c:        'b',
			wantKey:  "bar",
		},
		{
			name:     "two-found-1",
			index:    []byte{'b', 'f'},
			children: []*nodeHeader{testMakeLeaf(txn, "bar"), testMakeLeaf(txn, "foo")},
			c:        'f',
			wantKey:  "foo",
		},
		{
			name:     "two-not-found",
			index:    []byte{'b', 'f'},
			children: []*nodeHeader{testMakeLeaf(txn, "bar"), testMakeLeaf(txn, "foo")},
			c:        'a',
			wantKey:  "",
		},
		{
			name:  "full-found-0",
			index: []byte{0x0, 'b', 'f', 0xFF},
			children: []*nodeHeader{
				testMakeLeaf(txn, "\x00\x00\x00"),
				testMakeLeaf(txn, "bar"),
				testMakeLeaf(txn, "foo"),
				testMakeLeaf(txn, "\x255\x255\x255"),
			},
			c:       0x0,
			wantKey: "\x00\x00\x00",
		},
		{
			name:  "full-found-1",
			index: []byte{0x0, 'b', 'f', 0xFF},
			children: []*nodeHeader{
				testMakeLeaf(txn, "\x00\x00\x00"),
				testMakeLeaf(txn, "bar"),
				testMakeLeaf(txn, "foo"),
				testMakeLeaf(txn, "\x255\x255\x255"),
			},
			c:       'b',
			wantKey: "bar",
		},
		{
			name:  "full-not-found",
			index: []byte{0x0, 'b', 'f', 0xFF},
			children: []*nodeHeader{
				testMakeLeaf(txn, "\x00\x00\x00"),
				testMakeLeaf(txn, "bar"),
				testMakeLeaf(txn, "foo"),
				testMakeLeaf(txn, "\x255\x255\x255"),
			},
			c:       'x',
			wantKey: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &node4{
				innerNodeHeader: innerNodeHeader{
					nodeHeader: nodeHeader{
						id:  1,
						typ: typNode4,
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

func TestNode4AddRemoveChild(t *testing.T) {
	txn := &Txn{}
	require := require.New(t)

	// Start with an empty node4
	n := txn.newNode4()
	// Sanity checks
	require.Equal(0, int(n.nChildren))
	require.Equal(typNode4, n.typ)

	// Add child 1
	nh := n.addChild(txn, 'f', testMakeLeaf(txn, "foo"))
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN := nh.node4()
	require.Exactly(gotN, n)
	require.Equal(1, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 'f', "foo")

	// Add child 2 that sorts before
	nh = n.addChild(txn, 0x0, testMakeLeaf(txn, "\x00\x00\x00"))
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(2, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 'f', "foo")
	assertChildHasLeaf(t, nh, 0x0, "\x00\x00\x00")

	// Add child 3 that sorts after
	nh = n.addChild(txn, 0xff, testMakeLeaf(txn, "\xff\xff\xff"))
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(3, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 'f', "foo")
	assertChildHasLeaf(t, nh, 0x0, "\x00\x00\x00")
	assertChildHasLeaf(t, nh, 0xff, "\xff\xff\xff")

	// Add child 4
	nh = n.addChild(txn, 'z', testMakeLeaf(txn, "zzz"))
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(4, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 'f', "foo")
	assertChildHasLeaf(t, nh, 0x0, "\x00\x00\x00")
	assertChildHasLeaf(t, nh, 0xff, "\xff\xff\xff")
	assertChildHasLeaf(t, nh, 'z', "zzz")

	// Save the node4 for later
	n4h := nh

	// Add child 5
	nh = n.addChild(txn, 'b', testMakeLeaf(txn, "bar"))
	// Should grow to a node16
	require.Equal(typNode16, nh.typ)
	require.Equal(5, int(nh.node16().nChildren))
	assertChildHasLeaf(t, nh, 'f', "foo")
	assertChildHasLeaf(t, nh, 0x0, "\x00\x00\x00")
	assertChildHasLeaf(t, nh, 0xff, "\xff\xff\xff")
	assertChildHasLeaf(t, nh, 'z', "zzz")
	assertChildHasLeaf(t, nh, 'b', "bar")

	// Remove from the n4 (remove from node16 is tested elsewhere)
	nh = n4h.removeChild(txn, 'f')
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(3, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 0x0, "\x00\x00\x00")
	assertChildHasLeaf(t, nh, 0xff, "\xff\xff\xff")
	assertChildHasLeaf(t, nh, 'z', "zzz")

	// Remove
	nh = n4h.removeChild(txn, 0x0)
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(2, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 0xff, "\xff\xff\xff")
	assertChildHasLeaf(t, nh, 'z', "zzz")

	// Remove
	nh = n4h.removeChild(txn, 0xff)
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(1, int(gotN.nChildren))
	assertChildHasLeaf(t, nh, 'z', "zzz")

	nh = n4h.removeChild(txn, 'z')
	// Should not have replace the node typ
	require.Equal(typNode4, nh.typ)
	gotN = nh.node4()
	require.Exactly(gotN, n)
	require.Equal(0, int(gotN.nChildren))
}

func TestNode4MinMaxChild(t *testing.T) {
	tests := []struct {
		name             string
		children         []string
		wantMin, wantMax string
	}{
		{
			name:     "empty",
			children: []string{},
			wantMin:  "",
			wantMax:  "",
		},
		{
			name:     "full",
			children: []string{"foo", "bar", "\x00\x00\x00", "\xff\xff\xff"},
			wantMin:  "\x00\x00\x00",
			wantMax:  "\xff\xff\xff",
		},
		{
			name:     "one null",
			children: []string{"\x00\x00\x00"},
			wantMin:  "\x00\x00\x00",
			wantMax:  "\x00\x00\x00",
		},
		{
			name:     "one str",
			children: []string{"foo"},
			wantMin:  "foo",
			wantMax:  "foo",
		},
		{
			name:     "two str",
			children: []string{"foo", "aaa"},
			wantMin:  "aaa",
			wantMax:  "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := &Txn{}

			n := &txn.newNode4().nodeHeader
			for _, k := range tt.children {
				n = n.addChild(txn, k[0], testMakeLeaf(txn, k))
			}

			gotMin := n.minChild()
			assertLeafKey(t, gotMin, tt.wantMin)

			gotMax := n.maxChild()
			assertLeafKey(t, gotMax, tt.wantMax)
		})
	}
}

func TestNode4LowerBound(t *testing.T) {
	tests := []struct {
		name      string
		children  []string
		key       string
		wantLower string
	}{
		{
			name:      "empty",
			children:  []string{},
			key:       "foo",
			wantLower: "",
		},
		{
			name:      "full, match",
			children:  []string{"foo", "bar", "\x00\x00\x00", "\xff\xff\xff"},
			key:       "foo",
			wantLower: "foo",
		},
		{
			name:      "full, no match",
			children:  []string{"foo", "bar", "\x00\x00\x00", "\xff\xff\xff"},
			key:       "baa",
			wantLower: "bar",
		},
		{
			name:      "full, same lower upper",
			children:  []string{"foo", "bar", "\x00\x00\x00", "\xff\xff\xff"},
			key:       "car",
			wantLower: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := &Txn{}

			n := &txn.newNode4().nodeHeader
			for _, k := range tt.children {
				n = n.addChild(txn, k[0], testMakeLeaf(txn, k))
			}

			gotLower := n.lowerBound(tt.key[0])
			assertLeafKey(t, gotLower, tt.wantLower)
		})
	}
}
