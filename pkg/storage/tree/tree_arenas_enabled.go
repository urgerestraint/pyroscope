//go:build goexperiment.arenas

package tree

import (
	"arena"
	"bytes"
	"github.com/pyroscope-io/pyroscope/pkg/util/arenahelper"
	"sort"
)

func (t *Tree) InsertStackA(stack [][]byte, v uint64) {
	a := t.arena
	if a == nil {
		t.InsertStack(stack, v)
		return
	}
	n := t.root
	for j := range stack {
		n.Total += v
		n = n.insertA(a, stack[j])
	}
	// Leaf.
	n.Total += v
	n.Self += v
}

func (n *treeNode) insertA(a *arenahelper.ArenaWrapper, targetLabel []byte) *treeNode {
	i := sort.Search(len(n.ChildrenNodes), func(i int) bool {
		return bytes.Compare(n.ChildrenNodes[i].Name, targetLabel) >= 0
	})
	if i > len(n.ChildrenNodes)-1 || !bytes.Equal(n.ChildrenNodes[i].Name, targetLabel) {
		l := arena.MakeSlice[byte](a.Arena, len(targetLabel), len(targetLabel))
		copy(l, targetLabel)
		child := newNodeA(a, l)
		n.ChildrenNodes = arenahelper.AppendA(n.ChildrenNodes, child, a)
		copy(n.ChildrenNodes[i+1:], n.ChildrenNodes[i:])
		n.ChildrenNodes[i] = child
	}
	return n.ChildrenNodes[i]
}

func newNodeA(a *arenahelper.ArenaWrapper, label []byte) *treeNode {
	n := arena.New[treeNode](a.Arena)
	n.Name = label
	return n
}

func NewA(a *arenahelper.ArenaWrapper) *Tree {
	t := arena.New[Tree](a.Arena)
	t.root = newNodeA(a, nil)
	t.arena = a
	return t
	//return &Tree{
	//	root: newNode([]byte{}),
	//}
}