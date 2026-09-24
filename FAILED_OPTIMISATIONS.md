# Failed Performance Optimisations

Context: the router is a static-route map + radix-tree hybrid with a dense
handler arena. The workload used for all measurements is
`BenchmarkRouterGithubParams` (the GitHub API route set, param paths only,
one `Search` per iteration), plus `BenchmarkParamMissSingle` where noted.
Machine: Apple M2 Max, go1.26.6. Every number is zero allocs.

Baseline reference: **~40 ns/op** for `BenchmarkRouterGithubParams`.

---

## S1 — Child bitmap with popcount rank

### Idea
Replace the sorted linear scan over a node's children with a per-node
`[2]uint64` bitmap (`childMask`) plus a popcount rank:

- bit *b* set = a child whose fingerprint byte is *b* exists
- on a hit, the child's index = `onesCount(mask & (bit-1)) + word*lowCount`
- direct `children[ci]` access, no scan

### Why it looked promising
The profile showed the child-scan block at ~24% of samples (loop + child load
+ `b != c.b` branch). The GitHub tree's root node has 17 children, visited on
every request, so a linear scan appeared to be doing ~9 iterations of
compare/branch work per request.

### What happened
| Variant | ns/op |
|---|---|
| Baseline (scan, 96-byte node) | 40.0 |
| Rank on every node (first attempt) | 48.2 |
| Threshold hybrid, finder-loop restructure | 44.3 |
| Threshold hybrid, duplicated inline scan | 42.9 |
| Same, rank block dead (threshold 1000) | 41.8 |
| Isolation: plain scan + 112-byte node | 40.0 |

Every variant regressed. The isolation test proved the 16-byte struct growth
was free; the loss was entirely the rank machinery and its codegen tax.

### Why it failed
1. **Bounds check on the computed index.** The hot line was
   `nn.children[ci].node` (590ms of samples). `ci` is computed at runtime, so
   the compiler cannot elide the slice bounds check — unlike
   `for i := range nn.children`, where `i` is provably in range.
2. **Fixed rank cost exceeds the scan.** The mask load + `bits.OnesCount64` +
   multiply cost more than 1–3 predictable iterations of the scan. 95 of 105
   nodes with children have ≤4 children, so only the root (17 children) could
   ever benefit — and it doesn't.
3. **Codegen tax.** Even with the rank branch never taken, the duplicated
   descent block measured 41.8 ns vs 40.0 — the second path degraded the
   branch layout of the whole descent loop.

### Verdict
Rejected. The 24% "child scan" share was misleading: it is the sum of many
small, branch-predictor-friendly scans, not one slow fan-out node.

### Key lesson
Never trust a hotspot's share before checking the distribution of the data it
processes. The root's 17-child scan is well-predicted and not slow.

---

## N1 — Direct `[256]nodePtr` lookup table

### Idea
The successor to S1: map path byte directly to child node index with a
`[256]nodePtr` table (sentinel -1), avoiding S1's three failure points —
no popcount, no multiply, no bounds check (a byte index into a `[256]` array
is provably in range).

### What happened
Rejected before implementation.

### Why it failed
The table is 1 KiB per fan-out node (or an 8-byte pointer field on *every*
node, allocated only for fan-out nodes). The user judged the node-size growth
unacceptable for the router's memory layout.

### Verdict
Rejected: node size.

---

## N2 — `nextSlash` 4-byte SWAR tail

### Idea
`nextSlash` was 4.7% flat in the profile. The 8-byte SWAR loop handles middle
segments; the byte-at-a-time tail loop handles the last segment of every path
(typically 3–7 bytes). Add a 4-byte SWAR stage for the tail, and later a
byte-loop unroll×2.

### What happened
| Variant | `nextSlash` inlined? | Params | ParamMissSingle |
|---|---|---|---|
| Baseline | yes, 4 call sites | 40.4 ns | 17.06 ns |
| 4-byte SWAR tail | **no** | 41.5 ns (+1.1) | 17.6 ns (+0.5) |
| byte-loop unroll×2 | **no** | — | — |

### Why it failed
`nextSlash` sits exactly at Go's inline budget. Verified with `-gcflags=-m`:
the baseline emits `can inline nextSlash` and inlines it at all four call
sites; any code added (even the small unroll) removes the `can inline` line.
An un-inlined call costs ~1 ns per segment; the byte-tail scan it replaces is
worth less than that.

### Verdict
Rejected. `nextSlash` is already at its sweet spot — the 8-byte SWAR loop plus
the short byte tail fit exactly inside the inline budget, and the budget is
the binding constraint, not the tail scan.

---

## N4 — `methodToEnum` packed compare

### Idea
`methodToEnum` was 1.5% flat. Replace the string switch with a packed
`uint32` compare of the first four bytes, and later with a `switch len(method)`
dispatch.

### What happened
The assembly dump settled it before benchmarking:

```
CMP  $4, R1          ; length dispatch
BGT  ...
CMP  $3, R1
MOVWU (R0), R1       ; 4-byte load
MOVD $1145128264, R2 ; "HEAD" packed, little-endian
CMPW R2, R1
```

The compiler's string-switch codegen **already** emits word-level packed
compares (4-byte loads for len 4, `MOVHU` + byte check for len 3) with a
minimal length chain. The one remaining variant — hand-rolled
`switch len(method)` hoping for a jump-table length dispatch — measured worse:

| Variant | Params | ParamMissSingle |
|---|---|---|
| Baseline (compiler switch) | 39.8 ns | 17.08 ns |
| length-first switch | 41.1 ns (+1.3) | 17.13 ns |

### Why it failed
Go's string-switch lowering is already the optimal sound implementation for a
small constant set of short strings. Any sound hand-rolled version must verify
the full string, and the compiler's length partition + word compare is at
least as good as what can be written by hand.

### Verdict
Rejected. No source-level change beats the compiler here.

---

## L1 — Flat pointer-free node arena (104 → 32 byte node)

### Idea
Split the tree into a mutable build form (the existing `[]node`) and a
flattened search form compiled from it, on the theory that the descent was
bound by cache footprint:

- cold fields (`wildcard`, `catchAllName`, `catchAllNode`) moved to a parallel
  `extras` array keyed by node index, so a descent that never hits a wildcard
  never pulls them into cache
- `children []child` replaced by `childStart uint32` + `childLen uint16` into
  two global arenas, `childBytes []byte` and `childNodes []nodePtr`
- `prefix string` replaced by a `uint32` offset + `uint16` length into one
  concatenated `blob string`

Result: a 32-byte search node (asserted at compile time), two per cache line,
never straddling one, with zero pointers for the GC to scan in the hot arrays.
The build tree kept its existing shape, so `insert`/`remove` were untouched;
`Search` read only the compiled form.

### Why it looked promising
104 bytes per node means two cache lines touched per descent step, and the
GitHub tree at ~1000 nodes is ~104 KB — well past L1. A 3× smaller footprint
looked like it should pay for itself.

### What happened
| Benchmark | Baseline | Flat arena |
|---|---|---|
| `BenchmarkRouterGithub` | 33.5 ns | 37.4 ns |
| `BenchmarkRouterGithubParams` | 37.8 ns | 43.0 ns |
| `BenchmarkParamMissSingle` | 16.5 ns | 19.1 ns |
| cold-cache sweep (16 MiB thrash between sweeps) | ~12.5 µs | ~13.8 µs |
| `BenchmarkBuildGithubAPI` | 63 µs | 1.02 ms |

### The isolation test
Padding the **original** `node` from 104 to 148 bytes — a 42% growth — changed
nothing: 37.4 ns vs 37.8 ns on `BenchmarkRouterGithubParams`. Node footprint
is not on the critical path at all, warm or cold. The premise was simply
false, and every variant below was chasing a bottleneck that does not exist.

### Why it failed
1. **Footprint is not the bottleneck.** The tree is re-walked continuously, so
   the nodes it actually visits stay resident regardless of their size. This
   holds even with 16 MiB of thrash between sweeps: the flat form still lost.
2. **Fewer bytes, more instructions.** The baseline reads the `children` slice
   header from bytes already in the node's own cache line, and one
   `child{b, node}` load yields both the match byte and the target index. The
   flat form loads `childBytes` and `childNodes` headers off `*compiled`
   (separate cache lines from the node), indexes two arenas with two bounds
   checks, then reaches into `blob` for any prefix compare the 8-byte
   `prefixWord` path does not cover.
3. **Hoisting the arenas made it worse.** Binding `nodes`, `childBytes`,
   `childNodes` and `blob` to locals at the top of `search` to avoid reloading
   the slice headers measured 43.9 ns — register pressure across the `goto`
   web spilled them anyway, and cost more than it saved.
4. **The compile step is not free.** Reflattening on every mutation is a 16×
   build regression. Deferring it to the first `Search` after a mutation is a
   write from a reader, which needs an atomic pointer to be safe.

### Verdict
Rejected on every axis measured. Reverted in full.

### Key lesson
Prove a bottleneck is what you think it is before restructuring around it. A
one-line padding probe (grow the struct, measure) would have killed this idea
in two minutes; it was run last instead of first. Trading data size for
instruction count is a loss whenever the working set is already resident, and
for a router it always is.

---

## L2 — wildcard as struct-of-arrays in the node

### Idea
Keep the wildcard data on the node (no side table) but switch from an
array-of-structs to a struct-of-arrays:

```go
// before
wildcard []wildcard            // wildcard{params []string; node nodePtr; minRun uint8}

// after
wildcard struct{               // one node's entries as parallel arrays
    params [][]string
    nodes  []nodePtr
    minRun []uint8
}
```

Rationale: `canBacktrack`'s hot probe reads only `minRun[wi]`, and in the AoS
form each entry's `minRun` sits after the 16-byte `params` slice header at 32
bytes stride. SoA packs `minRun` into a dense `[]uint8` — one byte per entry —
and drops per-entry data from 32 to 21 bytes.

### Why it looked promising
The AoS entry wastes padding and scatters the three fields; SoA looked like
cheaper scans and a denser wildcard heap for the same code shape (no side
table, no index remapping).

### What happened
| Benchmark | Baseline | SoA wildcard |
|---|---|---|
| `BenchmarkRouterGithubParams` | 39.6 ns | 40.1 ns (parity) |
| `BenchmarkParamMissSingle` | 17.4 ns | 17.4 ns (parity) |
| `BenchmarkBuildGithubAPIInsertOnly` | 65.7 µs / 1,061 allocs | 75.3 µs (+15%) / 1,289 allocs |
| `MemSize` (full GitHub tree) | 79,965 B | 110,581 B (+38%) |
| node size | 104 B | 152 B (+48 B) |

### Why it failed
1. **Three slice headers on every node.** The SoA struct is 72 bytes of slice
   headers vs 24 for the old `[]wildcard` header — +48 B per node, paid even
   by nodes that have zero wildcards (three nil headers instead of one). That
   is the entire +38% MemSize.
2. **The per-entry savings are irrelevant.** Data per entry drops 32 → 21 B,
   but only wildcard-bearing nodes have entries, and those are few. Most
   nodes only ever paid the header, and SoA tripled it.
3. **Build got slower, not faster.** One append to `[]wildcard` became three
   appends to independently-growing arrays: +15% build time, 1,289 vs 1,061
   allocs (each array doubles separately).
4. **No hot-path gain.** The profile is indistinguishable from baseline:
   `search` still ~63% flat, `minRun` probe still a single load. The dense
   `[]uint8` was never a bottleneck to begin with.

### Verdict
Rejected: a pure memory and build regression with no measured perf effect.
The same SoA data in a side table (off-node) avoids the header bloat but was
already covered by L1's verdict — footprint is not on the critical path.

### Key lesson
Structure-of-arrays only pays when the hot scan touches many entries of the
same field. Here each node has ≤ a handful of wildcard entries, and the hot
read is one element — so the dense `[]uint8` gained nothing while the extra
slice headers cost 48 bytes on every node. Moving a struct's *shape* around
does not move its *cost* unless the headers stop living on the common path.

---

## P1 — Profile-guided optimization (`-pgo`)

### Idea
Zero-source-change win: build with `-pgo` and a CPU profile. Every prior
restructure failed on branch layout, which is exactly what PGO tunes from
real branch frequencies.

### What happened
Tested with two profiles: the mixed `cpu.out` (parallel-bench heavy) and a
clean serial profile of the three target benchmarks (go1.26.6).

| Variant | Github | GithubParams | ParamMissSingle |
|---|---|---|---|
| Baseline | 31.5 ns | 36.1 ns | 16.1 ns |
| PGO, mixed profile | +3.4% | +2.6% | −5.3% |
| PGO, clean serial profile | +5.7% | +3.5% | −7.0% |

### Why it failed
PGO's inlining/layout decisions perturb the hand-tuned descent loop the same
way every manual restructure did: the hit path (the one that matters) regresses
while only the early-exit miss path improves. The loop is already laid out
better than PGO's model for this workload.

### Verdict
Rejected. Do not ship `default.pgo`; a downstream app profiling its whole
binary may still benefit, but the router itself loses on hits.

---

## P2 — 32 → 16-byte `searchFrame` (halve frameStack zeroing)

### Idea
`var stack frameStack` zeroing showed 270ms flat (~1.5% of search cum).
Shrink `searchFrame` fields (`idx`/`wi` → `int32`, `paramsIndex` → `int32`)
so the 4-frame inline array zeroes 64B instead of 128B.

### What happened
| Variant | Github | GithubParams | ParamMissSingle |
|---|---|---|---|
| Baseline | 31.5 ns | 36.1 ns | 16.1 ns |
| 16-byte frame | +2.7% | +2.4% | +2.1% |

### Why it failed
The `int32(idx)` / `int(f.idx)` conversions sit on push/pop hot sites, and the
narrower stores change codegen in the descent loop. The 64B less zeroing is
worth well under the conversion + layout cost. Same lesson as the descent-head
restructures: the 270ms on the `var stack` line is partly skid, not all real
zeroing cost.

### Verdict
Rejected.

---

## P3 — Tree-only search (delete the static map)

### Idea
The static-map miss costs param paths ~7% of the profile (`staticKey` +
length filter + `mapaccess2_faststr` on every param-path Search, since param
paths share lengths with static routes). Insert static routes into the radix
tree, delete the map, `staticLen`, `staticKey`, `normalizeStaticPath` — one
lookup structure, param paths skip the hash entirely.

### What happened
Minimal variant: static Adds routed through `splitPath` + `insert`, map
lookup removed from Search. All routing tests pass.

| Bench | Baseline | Tree-only |
|---|---|---|
| RouterGithub (static hits) | 32.9 ns | +4.9% |
| RouterGithubParams | 37.5 ns | **+1.8%** |
| RouterGithubAll | 7.78 µs | +5.6% |
| RouterGithubRandom | 7.64 µs | +9.6% |
| ParamMissSingle | 16.5 ns | +4.6% |
| RouterLarge (static-heavy) | 25.5 ns | **+137%** |

### Why it failed
The predicted ~2.5 ns win on param paths never materialised — param paths got
*slower* (+1.8%). Folding ~150 GitHub static routes into the tree inflates
fan-out and node count along exactly the prefixes param paths descend
(`/repos/...`, `/users/...`), so every param Search pays more children-scan
iterations and cache misses than the map probe it saved. Static hits lose
outright (hash is O(1)-ish in depth; descent is not), and RouterLarge shows
the worst case: long static paths go from one hash to a deep walk.

### Verdict
Rejected on all fronts — not even workload-dependent. The hybrid map+tree
split is load-bearing: it keeps the tree small and param-only, which is what
keeps the child scans branch-predictable (S1's lesson). The map is not
overhead on param paths; it is what keeps the tree cheap.

---

## N5 — `[256]int16` byte-dispatch table on fan-out nodes

### Idea
The untested middle ground between S1 (popcount rank, regressed) and N1
(rejected on node size): give a direct byte-indexed child table only to nodes
with many children, so the table cost is paid by a handful of nodes instead of
all of them. A byte index into a `[256]` array is provably in range, so there
is no bounds check and no popcount — S1's two failure points both gone.

Fan-out distribution for the GitHub tree (375 nodes total):

| children | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 11 | 17 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| nodes | 271 | 44 | 38 | 9 | 2 | 1 | 1 | 4 | 1 | 1 | 2 | 1 |

At a threshold of 8 children, 6 nodes get a table.

### What happened
Four variants, each isolated against a dead-table control:

| Variant | Github | GithubParams | ParamMissSingle |
|---|---|---|---|
| Baseline | 32.51 ns | 37.35 ns | 16.50 ns |
| (a) unified finder, `cnode >= 0` test, tables dead | +11.35% | +10.76% | +12.49% |
| (a) unified finder, tables live | +2.65% | +1.93% | +11.85% |
| (b) duplicated match block, tables dead | +5.81% | +6.10% | +4.33% |
| (b) duplicated match block, tables live | ~0% | −1.49% | +4.09% |
| (c) branchless via arena dead node, tables live | +10.38% | +10.25% | +22.25% |
| (d) duplicated + `flagHasTable` gate + out-of-line block | **−1.03%** | **−1.85%** | +4.76% |

Variant (d) is the best form. Its wider numbers:

| Benchmark | Baseline | Variant (d) |
|---|---|---|
| `RouterGithubAll` | 7.701 µs | −1.12% |
| `RouterGithubRandom` | 7.672 µs | −0.99% |
| `BuildGithubAPI` | 54.55 µs | **+40.04%** |

`RouterLarge` could not be measured — `largeAPI` is empty in the repo.

### Why it failed
1. **The table itself works; getting to it does not.** Comparing each variant
   against its own dead-table control isolates this cleanly: the table is worth
   roughly 6% on the nodes that have it, but every mechanism for choosing
   between the table and the scan costs 4–12% everywhere else.
2. **The unified finder is the expensive part, not the table.** Variant (a)
   replaced the fused scan/descent block with "find the child index, then
   match" — the same restructure the descent-head experiments already
   punished. It cost 11% with the table completely dead.
3. **A branchless dead node is worse than an early exit.** Variant (c) removed
   the miss test by pointing absent bytes at an unmatchable arena node, so a
   miss ran the whole match body (prefix word compare, trailing-slash check)
   before falling through to wildcards. `ParamMissSingle` regressed 22%: the
   scan loop's `break` on a miss is load-bearing.
4. **The remaining 5% is one branch in the descent head.** Variant (d) is the
   baseline scan verbatim plus a `flagHasTable` test, with the table block moved
   below the wildcards section so the scan keeps its layout. It still costs
   +4.76% on `ParamMissSingle` — about 0.8 ns, on a path where the tree has no
   table at all. Moving the block out of line did not help, and putting the
   table in `nodeCold` instead of on `node` did not either (−0.75% / −1.63% /
   +5.27%), confirming this is branch layout in the descent head and not node
   size. Same conclusion the descent-head restructures reached.
5. **Build regressed 40%.** `refreshDispatch` rescans every node and re-zeroes
   512 bytes per table on every `Add`. Fixable by clearing only the previously
   set bytes, but not worth writing for a 1% hit-path gain.

### Verdict
Rejected. The best variant is a real but ~1% gain on hit paths, bought with
~5% on the miss path and a 40% build regression. Nowhere near the 30 ns target
it was probing for: `RouterGithub` 32.51 → 32.17 ns.

### Key lesson
The descent head is a local optimum with no room for an extra branch. Three
separate attempts now (S1's rank, the single-load restructures, this table)
have each found a real improvement inside the child lookup and then lost more
than they gained on the branch that selects it. Any future win here has to
change the descent's *shape* — fewer dependent loads per request — not the
child lookup inside the existing shape.

---

## Summary

| Change | Result | Reason |
|---|---|---|
| S1 bitmap + popcount rank | +8 ns (regression) | bounds check on computed index + fixed rank cost |
| N1 `[256]nodePtr` table | rejected | node size |
| N2 nextSlash tail SWAR / unroll | +1 ns (regression) | breaks `nextSlash` inlining |
| N4 methodToEnum packed / length switch | +1 ns (regression) | compiler already optimal |
| L1 flat pointer-free node arena | +4 to +5 ns (regression), 16× build | footprint is not the bottleneck; more instructions per step |
| L2 wildcard struct-of-arrays | 0 ns (parity), +38% memory, +15% build | three slice headers on every node; dense `minRun` never a bottleneck |
| Pointerless `param` (interned name indices, kills `Params.set` write barrier) | +1.6 to +3.6% (regression) | barrier was skid-cheap; per-Search names-table handoff and extra `Get` indirection cost more than the 24→12B entry write saved |
| Sorted-children early `break` in search scan | ~0% hits, +2.7% ParamMissSingle (regression) | fans are small (2-8); extra compare per element costs more than early exit saves |
| Single-load descent head (goto step / hoisted byte / nested-if, 3 variants) | +2 to +6% (regression, all variants) | profile blamed the duplicate `path[idx]` load on lines 427+440 (3.45s), but the cost is loop-head branch + node cache misses skidding onto nearby loads; the second load is L1-hot and free, while every restructure worsened branch layout |
| P1 PGO build (`-pgo`, mixed + clean profiles) | +2.6 to +5.7% on hits (regression), −5 to −7% on miss | PGO layout perturbs the hand-tuned descent loop; hit path loses |
| P2 16-byte `searchFrame` (int32 fields, halve stack zeroing) | +2.1 to +2.7% (regression) | int↔int32 conversions on push/pop hot sites cost more than 64B less zeroing; `var stack` line was partly skid |
| N5 `[256]int16` dispatch table on fan-out nodes (>= 8 children) | −1.0 to −1.9% hits, +4.8% miss, +40% build | table wins ~6% where present, but the `flagHasTable` branch in the descent head costs ~5% on every table-less path |
| P3 tree-only search (static routes into tree, map deleted) | +1.8% params, +4.9% static, +137% RouterLarge (regression, all benches) | static routes inflate fan-out on the prefixes param paths descend; map probe saved less than the bigger tree costs; hybrid split is load-bearing |

Successful optimisations (for contrast):

- **S2** — first-8-bytes `prefixWord` masked compare: `memequal` 7.1% → 5.4%.
- **N3** — remove the `matched` intermediate in the prefix compare:
  −0.5 to −0.8 ns across benchmarks.
- **Go map swap** — +1.1 ns, accepted deliberately for ~200 fewer lines of
  custom hash-table code.
- **Child SoA fusion** — `childBytes`/`childNodes` fused into one
  `children []childRef{b byte; n nodePtr}`: perf parity (Github ~, All +0.8%,
  Params −0.4%), accepted for simplicity — one slice, no parallel-array
  invariant, node 24B smaller.
- **Short-tail word compare** — when fewer than 8 bytes remain, load the 8
  bytes ending at the prefix's end and shift down instead of `memequal`; same
  for the missing-trailing-slash check. All −2.5%, Random −2.6%, Params
  −2.4%, Parse +2.4%. The trailing-slash half carries the win (GitHub
  requests end without `/`, stored tails end with it); the prefix half alone
  is neutral to slightly negative. Kept both.
- **Per-method search start** — when the root's only way forward is its `/`
  child (no handler, wildcard or catch-all), start search there at idx 1,
  recomputed on `Add`/`Remove`. Skips a loop pass that could only step into
  `/`, with no new branch in the loop. All −5.9%, Random −5.6%, Params
  −5.0%, Parse −2.4%, Github and ParamMissSingle ~. Parallel was noise
  (+6.8% then −6.0% interleaved).

### Remaining profile costs (all structural, no cheap lever found)

- child scan ~24% (S1/N1/N5 failed)
- static-map miss ~7% (param paths share first segments with static routes;
  tree-only search tested in P3 — regressed everything, the map is load-bearing)
- terminal check ~5% (already the minimal bounds+trailing-slash gate)
- `nextSlash` ~5% (inline budget binding)
- `Params.set` ~3% (API contract: 32-byte write per param)
