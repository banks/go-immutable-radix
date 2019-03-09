package art

import (
	"bytes"
	"fmt"
	"strings"
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
}

func (d *dumper) String() string {
	d.buf = bytes.NewBufferString("")
	d.dumpNode(d.root)
	return d.buf.String()
}

func (d *dumper) padding() (string, string) {
	depth := len(d.nChildStack)
	if depth == 0 {
		return "───", "   "
	}
	pad := "    "
	pad += strings.Repeat("│  ", depth-1)

	currentLevelChildrenLeft := d.nChildStack[len(d.nChildStack)-1]

	head := "├──"
	finalPad := "│  "
	if currentLevelChildrenLeft == 1 {
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

func (d *dumper) dumpNode(n *nodeHeader) {
	headerPad, pad := d.padding()

	switch n.typ {
	case typLeaf:
		// Leaf node!
		leaf := n.leafNode()
		fmt.Fprintf(d.buf, "%s Leaf (%p)\n", headerPad, leaf)
		fmt.Fprintf(d.buf, "%s id:     %d\n", pad, n.id)
		fmt.Fprintf(d.buf, "%s prefix: %q\n", pad, string(n.prefix[0:n.prefixLen]))
		fmt.Fprintf(d.buf, "%s key:    %q\n", pad, leaf.key)
		fmt.Fprintf(d.buf, "%s val:    %v\n", pad, leaf.value)
	case typNode4:
		n4 := n.node4()
		fmt.Fprintf(d.buf, "%s Node4 (%p)\n", headerPad, n4)
		fmt.Fprintf(d.buf, "%s id:         %d\n", pad, n.id)
		fmt.Fprintf(d.buf, "%s prefix:     %q\n", pad, string(n.prefix[0:n.prefixLen]))
		fmt.Fprintf(d.buf, "%s nullIsLeaf: %v\n", pad, n.nullByteIsLeaf)
		fmt.Fprintf(d.buf, "%s index:      ", pad)
		d.dumpIndexArray(n4.index[:], int(n.nChildren))
		d.buf.WriteRune('\n')
		fmt.Fprintf(d.buf, "%s children: %v\n", pad, n4.children[:])

		d.pushNChildren(int(n.nChildren))

		for _, child := range n4.children[0:n.nChildren] {
			if child != nil {
				d.dumpNode(child)
				d.decNChildren()
			}
		}

		d.popNChildren()

	case typNode16:

	case typNode48:

	case typNode256:
	}

}
