package art

import (
	"bytes"
	"fmt"
)

// dumper outputs a string representation of the ART for debugging.
//
// For an ART with keys/values [A, a, aa, bar] it would output:
//
//   ─── node4 (0xc8200f3b30)
// 		   prefix: ""
// 		   index: [A a b ·]
// 		   children(3): [0xc8200f3af0 0xc8200f3b70 0xc8200f50a0 <nil>]
// 		   ├── Leaf (0xc8200f3af0)
// 		   │   key: "A"
// 		   │   val: "A"
// 		   │
// 		   ├──  node4 (0xc8200f3b70)
// 		   │   prefix: ""
// 		   │   index: [97 · · ·]
// 		   │   children(2): [0xc8200f3b20 0xc8200f3b60 <nil> <nil>]
// 		   │   ├── Leaf (0xc8200f3b20)
// 		   │   │   key: [97] [a]
// 		   │   │   val: a
// 		   │   │
// 		   │   └── Leaf (0xc8200f3b60)
// 		   │       key: [97 97] [aa]
// 		   │       val: aa
// 		   │
// 		   └── Leaf (0xc8200f50a0)
// 		       key: "bar"
// 		       val: "bar"
type dumper struct {
	root        *nodeHeader
	buf         *bytes.Buffer
	nChildStack []int
	innerLeaf   bool
}

// Dump returns the human readable debug output of a radix node and it's
// children recursively.
func Dump(root *nodeHeader) string {
	d := &dumper{root: root}
	return d.String()
}

func (d *dumper) String() string {
	d.buf = bytes.NewBufferString("")
	d.dumpNode(d.root)
	return d.buf.String()
}

func (d *dumper) isLastChild() bool {
	if len(d.nChildStack) < 1 {
		return true
	}
	return d.nChildStack[len(d.nChildStack)-1] == 1
}

func (d *dumper) padding() (string, string) {
	depth := len(d.nChildStack)
	if depth == 0 {
		return "───", "   "
	}
	pad := "    "
	for i := 0; i < depth-1; i++ {
		if d.nChildStack[i] > 1 {
			pad += "│   "
		} else {
			pad += "    "
		}
	}

	head := "├──"
	finalPad := "│  "
	if d.isLastChild() {
		head = "└──"
		finalPad = "   "
	}
	return pad + head, pad + finalPad
}

func (d *dumper) pushNChildren(n int) {
	d.nChildStack = append(d.nChildStack, n)
}

func (d *dumper) decNChildren() {
	if len(d.nChildStack) < 1 {
		return
	}
	d.nChildStack[len(d.nChildStack)-1]--
}

func (d *dumper) popNChildren() {
	depth := len(d.nChildStack)
	if depth > 0 {
		d.nChildStack = d.nChildStack[0 : depth-1]
	}
}

func (d *dumper) dumpIndexArray(a []byte, n int) {
	d.buf.WriteRune('[')
	for i, c := range a {
		if i < n {
			fmt.Fprintf(d.buf, "%q", c)
		} else {
			d.buf.WriteRune('·')
		}
		if i < len(a)-1 {
			d.buf.WriteRune(' ')
		}
	}
	d.buf.WriteRune(']')
}

func (d *dumper) dumpIndex48(a []byte) {
	d.buf.WriteRune('{')
	for i := 0; i < 256; i++ {
		if a[i] != 0x0 {
			fmt.Fprintf(d.buf, " %q:%d", byte(i), a[i]-1)
		}
	}
	d.buf.WriteString(" }")
}

func (d *dumper) dumpIndex256(a []*nodeHeader) {
	d.buf.WriteRune('{')
	for i := 0; i < 256; i++ {
		if a[i] != nil {
			fmt.Fprintf(d.buf, " %q", byte(i))
		}
	}
	d.buf.WriteString(" }")
}

func (d *dumper) dumpInnerNode(pad string, n *innerNodeHeader) {
	fmt.Fprintf(d.buf, "%s id:         %d\n", pad, n.id)
	fmt.Fprintf(d.buf, "%s prefix(%d): %q\n", pad, n.prefixLen,
		string(n.prefix[0:minU16(n.prefixLen, maxPrefixLen)]))
	if n.leaf == nil {
		fmt.Fprintf(d.buf, "%s innerLeaf:  nil\n", pad)
	} else {
		fmt.Fprintf(d.buf, "%s innerLeaf:\n", pad)
		d.innerLeaf = true
		d.pushNChildren(2) // make lines continue through the node
		d.dumpNode(&n.leaf.nodeHeader)
		d.popNChildren()
		d.innerLeaf = false
	}
}

func (d *dumper) dumpChildren(pad string, nChildren int, children []*nodeHeader) {
	fmt.Fprintf(d.buf, "%s children: %v\n", pad, children)

	d.pushNChildren(nChildren)

	for _, child := range children[0:nChildren] {
		if child != nil {
			d.dumpNode(child)
			d.decNChildren()
		}
	}

	d.popNChildren()
}

func (d *dumper) dumpNode(n *nodeHeader) {
	headerPad, pad := d.padding()

	switch n.typ {
	case typLeaf:
		// Leaf node!
		leaf := n.leafNode()
		fmt.Fprintf(d.buf, "%s Leaf (%p)\n", headerPad, leaf)
		fmt.Fprintf(d.buf, "%s id:     %d\n", pad, n.id)
		fmt.Fprintf(d.buf, "%s key:    %q\n", pad, leaf.key)
		fmt.Fprintf(d.buf, "%s val:    %v\n", pad, leaf.value)
		if !d.innerLeaf {
			fmt.Fprintf(d.buf, "%s\n", pad)
		}

	case typNode4:
		n4 := n.node4()
		fmt.Fprintf(d.buf, "%s Node4 (%p)\n", headerPad, n4)
		d.dumpInnerNode(pad, &n4.innerNodeHeader)
		fmt.Fprintf(d.buf, "%s index:      ", pad)
		d.dumpIndexArray(n4.index[:], int(n4.nChildren))
		d.buf.WriteRune('\n')
		d.dumpChildren(pad, int(n4.nChildren), n4.children[:])

	case typNode16:
		n16 := n.node16()
		fmt.Fprintf(d.buf, "%s Node16 (%p)\n", headerPad, n16)
		d.dumpInnerNode(pad, &n16.innerNodeHeader)
		fmt.Fprintf(d.buf, "%s index:      ", pad)
		d.dumpIndexArray(n16.index[:], int(n16.nChildren))
		d.buf.WriteRune('\n')
		d.dumpChildren(pad, int(n16.nChildren), n16.children[:])

	case typNode48:
		n48 := n.node48()
		fmt.Fprintf(d.buf, "%s Node48 (%p)\n", headerPad, n48)
		d.dumpInnerNode(pad, &n48.innerNodeHeader)
		fmt.Fprintf(d.buf, "%s index:      ", pad)
		d.dumpIndex48(n48.index[:])
		d.buf.WriteRune('\n')
		d.dumpChildren(pad, int(n48.nChildren), n48.children[:])

	case typNode256:
		n256 := n.node256()
		fmt.Fprintf(d.buf, "%s Node256 (%p)\n", headerPad, n256)
		d.dumpInnerNode(pad, &n256.innerNodeHeader)
		fmt.Fprintf(d.buf, "%s index:      ", pad)
		d.dumpIndex256(n256.children[:])
		d.buf.WriteRune('\n')
		d.dumpChildren(pad, 256, n256.children[:])
	}
}
