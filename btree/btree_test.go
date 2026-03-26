package btree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestBNodeExample(t *testing.T) {
	bNodeExample()
}

func bNodeExample() {
	node := BNode(make([]byte, BTREE_PAGE_SIZE))
	node.setHeader(LeafBNode, 2)
	nodeAppendKV(node, 0, 0, []byte("k1"), []byte("hi"))
	nodeAppendKV(node, 1, 0, []byte("k3"), []byte("hello"))
	printBNodeDebug(node)

	// lookup key with an update
	key := []byte("k2")
	val := []byte("hello2")
	new := BNode(make([]byte, BTREE_PAGE_SIZE))

	idx := nodeLookupLE(node, key)
	if bytes.Equal(key, node.getKey(idx)) {
		// found, update it
		leafUpdate(new, node, idx, key, val)
	} else {
		// not found. insert
		leafInsert(new, node, idx+1, key, val)
	}

	printBNodeDebug(node)
}

func init() {
	node1max := 4 + 1*8 + 1*2 + 4 + BTREE_MAX_KEY_SIZE + BTREE_MAX_VAL_SIZE
	assert(node1max <= BTREE_PAGE_SIZE) // maximum KV
}

func printBNodeDebug(node BNode) {
	bnodeType := map[uint16]string{
		InternalBNode: "InternalBNode",
		LeafBNode:     "LeafBNode",
	}

	data := []byte(node)

	nodeType := node.btype()
	numKeys := node.nkeys()

	fmt.Printf("--- BNode Dump ---\n")
	fmt.Printf("Type: %s | NumKeys: %d\n", bnodeType[nodeType], numKeys)

	for i := range numKeys {
		cellOff := node.kvPos(i)

		cursor := int(cellOff)

		keyLen := int(binary.LittleEndian.Uint16(data[cursor : cursor+2]))
		cursor += 2

		valLen := int(binary.LittleEndian.Uint16(data[cursor : cursor+2]))
		cursor += 2

		keyBytes := data[cursor : cursor+keyLen]
		cursor += keyLen

		valBytes := data[cursor : cursor+valLen]

		fmt.Printf("  [%d] Key: %-10s Val: %-10s (RawOff: %d)\n",
			i, string(keyBytes), string(valBytes), cellOff)
	}
	fmt.Println("------------------")
}
