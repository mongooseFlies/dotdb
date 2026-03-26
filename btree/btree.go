// Package btree implements a B-Tree data structure.
package btree

import (
	"bytes"
	"encoding/binary"
)

/*
	 NOTE:
# node:
	| type | nkeys |  pointers  |  offsets   | key-values | unused |
	|  2B  |   2B  | nkeys × 8B | nkeys × 2B |     ...    |        |
# KV pair:
	| key_size | val_size | key | val |
	|    2B    |    2B    | ... | ... |
*/

type Node struct {
	keys [][]byte
	// one of the following, the other is nil
	vals     [][]byte // for leaf nodes only
	children []*Node  // for internal nodes only
}

func Encode(node Node) []byte           { return nil }
func Decode(page []byte) (*Node, error) { return nil, nil }

const (
	// NOTE: same as in OS page size
	BTREE_PAGE_SIZE    = 4096
	BTREE_MAX_KEY_SIZE = 1000
	BTREE_MAX_VAL_SIZE = 3000
)

const (
	InternalBNode = iota
	LeafBNode
)

// BNode --> Btree representation on [primarly] disk and memory
type BNode []byte

func (node BNode) btype() uint16 {
	return binary.LittleEndian.Uint16(node[0:2])
}

func (node BNode) nkeys() uint16 {
	return binary.LittleEndian.Uint16(node[2:4])
}

func (node BNode) setHeader(btype uint16, nkeys uint16) {
	binary.LittleEndian.PutUint16(node[0:2], btype)
	binary.LittleEndian.PutUint16(node[2:4], nkeys)
}

// read the child pointers array
func (node BNode) getPtr(idx uint16) uint64 {
	assert(idx < node.nkeys())
	pos := 4 + 8*idx
	return binary.LittleEndian.Uint64(node[pos:])
}

// write the child pointers array
func (node BNode) setPtr(idx uint16, val uint64) {
	assert(idx < node.nkeys())
	pos := 4 + 8*idx
	binary.LittleEndian.PutUint64(node[pos:], val)
}

// read the `offsets` array --> each offset = 2B
func (node BNode) getOffset(idx uint16) uint16 {
	if idx == 0 {
		return 0
	}
	pos := 4 + 8*node.nkeys() + 2*(idx-1)
	return binary.LittleEndian.Uint16(node[pos:])
}

func (node BNode) kvPos(idx uint16) uint16 {
	assert(idx <= node.nkeys())
	return 4 + 8*node.nkeys() + 2*node.nkeys() + node.getOffset(idx)
	// NOTE:       ↓               ↓                    ↓
	//    ↓        ↓               ↓                    ↓
	//  [headers]  ↓               ↓                [KV position]
	//         [num-keys]	      [offsets]
}

func (node BNode) getKey(idx uint16) []byte {
	assert(idx < node.nkeys())
	pos := node.kvPos(idx)
	klen := binary.LittleEndian.Uint16(node[pos:])
	return node[pos+4:][:klen]
}

func (node BNode) getVal(idx uint16) []byte {
	assert(idx < node.nkeys())
	pos := node.kvPos(idx)
	klen := binary.LittleEndian.Uint16(node[pos+0:])
	vlen := binary.LittleEndian.Uint16(node[pos+2:])
	return node[pos+4+klen:][:vlen]
}

func (node BNode) setOffset(idx uint16, val uint16) {
	assert(idx > 0) // offset[0] is implicit (always 0)
	pos := 4 + 8*node.nkeys() + 2*(idx-1)
	binary.LittleEndian.PutUint16(node[pos:], val)
}

// node size in bytes
func (node BNode) nbytes() uint16 {
	return node.kvPos(node.nkeys())
}

func leafInsert(
	new BNode, old BNode, idx uint16, key []byte, val []byte,
) {
	new.setHeader(LeafBNode, old.nkeys()+1)
	nodeAppendRange(new, old, 0, 0, idx)                   // copy the keys before `idx`
	nodeAppendKV(new, idx, 0, key, val)                    // the new key
	nodeAppendRange(new, old, idx+1, idx, old.nkeys()-idx) // keys from `idx`
}

func leafUpdate(
	new BNode, old BNode, idx uint16, key []byte, val []byte,
) {
	new.setHeader(LeafBNode, old.nkeys())
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, 0, key, val)
	nodeAppendRange(new, old, idx+1, idx+1, old.nkeys()-(idx+1))
}

// copy multiple keys, values, and pointers into the position
func nodeAppendRange(
	new BNode, old BNode, dstNew uint16, srcOld uint16, n uint16,
) {
	for i := range n {
		dst, src := dstNew+i, srcOld+i
		nodeAppendKV(new, dst,
			old.getPtr(src), old.getKey(src), old.getVal(src))
	}
}

func nodeAppendKV(node BNode, idx uint16, ptr uint64, key []byte, val []byte) {
	node.setPtr(idx, ptr)
	// KVs
	pos := node.kvPos(idx) // uses the offset value of the previous key
	// 4-bytes KV sizes
	binary.LittleEndian.PutUint16(node[pos+0:], uint16(len(key)))
	binary.LittleEndian.PutUint16(node[pos+2:], uint16(len(val)))
	// KV data
	copy(node[pos+4:], key)
	copy(node[pos+4+uint16(len(key)):], val)
	// update the offset value for the next key
	node.setOffset(idx+1, node.getOffset(idx)+4+uint16((len(key)+len(val))))
}

// find the last postion that is less than or equal to the key
func nodeLookupLE(node BNode, key []byte) uint16 {
	nkeys := node.nkeys()
	var i uint16
	for i = 0; i < nkeys; i++ {
		cmp := bytes.Compare(node.getKey(i), key)
		if cmp == 0 {
			return i
		}
		if cmp > 0 {
			return i - 1
		}
	}
	return i - 1
}

// Split an oversized node into 2 nodes. The 2nd node always fits.
func nodeSplit2(left, right, old BNode) {
	assert(old.nkeys() >= 2)
	// the initial guess
	nleft := old.nkeys() / 2
	// try to fit the left half
	left_bytes := func() uint16 {
		return 4 + 8*nleft + 2*nleft + old.getOffset(nleft)
	}
	for left_bytes() > BTREE_PAGE_SIZE {
		nleft--
	}
	assert(nleft >= 1)
	// try to fit the right half
	right_bytes := func() uint16 {
		return old.nbytes() - left_bytes() + 4
	}
	for right_bytes() > BTREE_PAGE_SIZE {
		nleft++
	}
	assert(nleft < old.nkeys())
	nright := old.nkeys() - nleft
	// new nodes
	left.setHeader(old.btype(), nleft)
	right.setHeader(old.btype(), nright)
	nodeAppendRange(left, old, 0, 0, nleft)
	nodeAppendRange(right, old, 0, nleft, nright)
	// NOTE: the left half may be still too big
	assert(right.nbytes() <= BTREE_PAGE_SIZE)
}

// split a node if it's too big. the results are 1~3 nodes.
func nodeSplit3(old BNode) (uint16, [3]BNode) {
	if old.nbytes() <= BTREE_PAGE_SIZE {
		// old = old[:BTREE_PAGE_SIZE]
		return 1, [3]BNode{old} // not split
	}
	left := BNode(make([]byte, 2*BTREE_PAGE_SIZE)) // might be split later
	right := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(left, right, old)
	if left.nbytes() <= BTREE_PAGE_SIZE {
		left = left[:BTREE_PAGE_SIZE]
		return 2, [3]BNode{left, right} // 2 nodes
	}
	leftleft := BNode(make([]byte, BTREE_PAGE_SIZE))
	middle := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(leftleft, middle, left)
	assert(leftleft.nbytes() <= BTREE_PAGE_SIZE)
	return 3, [3]BNode{leftleft, middle, right} // 3 nodes
}

func assert(condition bool) {
	if !condition {
		panic("Assertion failed")
	}
}
