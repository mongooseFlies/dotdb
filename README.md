# DotDB

SQL database implementation in Go.

> [!NOTE]
> for learning purposes only

### Node Layout

|        header (4B)        |    pointers     |    offsets     |  key-values  | unused |
|:--------------------------|:---------------|:--------------|:------------|:-------|
| type(2B) | nkeys(2B)     |   nkeys × 8B   | nkeys × 2B    |     ...     |        |
|    2     |       2         | 4 + 8*nkeys    | 4+8nkeys+2nkeys|            |        |

### KV Pair Layout

| key_size(2B) | val_size(2B) | key | val |
|:-------------|:-------------|:---|:----|
|      2       |      2       |  ...  | ... |

- **Node header**: `type` and `nkeys` (both 2B, little-endian)
- **Pointers**: `nkeys × 8B` (uint64, child page offsets for internal nodes)
- **Offsets**: `nkeys × 2B` (uint16, relative to KV start; offset[0] is always 0)
- **KV data**: key_size + val_size (2B each) + key + val

## TODO

- [ ] Create underlying B-Tree implementation
- [ ] Create SQL parser
- [ ] Create SQL interpreter
- [ ] Create SQL compiler
- [ ] Create SQL virtual machine
- [ ] Create SQL executor
- [ ] Create SQL planner
- [ ] Create SQL optimizer
