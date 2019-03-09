package art

// APINode is a public veneer that matches the public interface of iradix.Node
// for drop-in compatibility while abstracting the complications of the internal
// ART node types.
type APINode struct {
	h *nodeHeader
}

// func (n *APINode) Dump(prefix string) string {
// 	return "TODO"
// }

// func (n *APINode) Get(k []byte) (interface{}, bool) {
// }

// func (n *APINode) GetWatch(k []byte) (<-chan struct{}, interface{}, bool) {

// }

// func (n *APINode) Iterator() *Iterator {

// }

// func (n *APINode) LongestPrefix(k []byte) ([]byte, interface{}, bool) {

// }

// func (n *APINode) Maximum() ([]byte, interface{}, bool) {

// }

// func (n *APINode) Minimum() ([]byte, interface{}, bool) {

// }

// func (n *APINode) Walk(fn WalkFn) {

// }

// func (n *APINode) WalkPath(path []byte, fn WalkFn) {

// }

// func (n *APINode) WalkPrefix(prefix []byte, fn WalkFn) {

// }
