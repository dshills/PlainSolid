# Nonconvex planar Boolean operations

`Combine(ctx, a, b, operation, Options{})` supports union, intersection and
ordered difference of closed planar `brep.Solid` boundaries. It uses an exact
rational BSP over each operand's oriented tessellation, with no convex
decomposition requirement. The result contains triangular planar faces.

Material boundaries must be embedded manifolds. Disconnected components,
cavities and islands inside cavities are supported; shell containment depth
must agree with shell winding. Input and output receive exact geometric
self-intersection checks. Touching configurations that would produce an
edge-only or vertex-only connection fail explicitly rather than returning a
nonmanifold solid. Empty results are represented by `brep.Solid{}`.

The exact geometry is the finite float64 **tessellation**, not an idealized
surface behind approximate face coordinates. Plane classification, splitting,
and restoration of shared edge subdivision use `math/big.Rat`. Face holes are
handled by the existing BRep tessellator. No tolerance is used to merge distinct
exact intersection vertices. Converting vertices back to float64 must preserve
distinct vertices, stay within `MaxConversionError`, and pass topology,
intersection, and shell-orientation checks. Rounding may slightly warp the
triangulated boundary; curved geometry and analytic face reconstruction are
outside this package. Tiny or extreme-scale features can be rejected by the
existing BRep/mesh validators even when exact splitting succeeds.

Default budgets:

| Option | Default |
| --- | ---: |
| MaxInputTriangles | 2,048 per operand |
| MaxFragments | 20,000 cumulative newly split polygons |
| MaxDepth | 128 BSP levels |
| MaxWork | 2,000,000 predicate/assembly work units |
| MaxVertices | 16,384 exact output vertices |
| MaxOutputTriangles | 8,192 |
| MaxBits | 4,096 bits per checked rational numerator/denominator |
| MaxCollisionPairs | 200,000 per validation pass |
| MaxConversionError | 1e-9 model units |

Intermediate rational expressions can temporarily exceed the checked stored
value size. Budgets bound operations and stored growth, not wall-clock time or
peak bytes exactly. Context cancellation is checked during BSP and assembly;
collision passes are individually bounded but not internally cancellable.
Shell containment retries 64 deterministic ray directions and returns an error
if all are degenerate. Errors never return a partial solid.

Evidence includes analytic aligned/shared-face/contained/disjoint cases, nested
cavities and islands, an independent oracle for two genuinely 3D nonconvex
branching solids under different dyadic shears, and randomized box identities.
The independent 3D oracle sums pairwise convex-cell intersections solely in the
test. These tests establish a useful bounded subset, not OCCT Boolean parity
or a proof for every pathological planar arrangement.

`CombineWithHistory` additionally returns one owning input operand, original
polygon face, tessellation triangle, and orientation-reversal flag for every
output triangular face. Per-input-triangle entries list output descendants and
classify deleted, retained unchanged, or fragmented/subdivided boundaries.
Coplanar overlap is attributed to one owning source; this is not an exhaustive
coincidence graph or persistent CAD naming. A deleted boundary does not mean
its enclosed material was removed. Failed operations return no partial history.

`SplitPlane` partitions a solid into negative and positive pieces using the exact
represented plane origin and normal. An enclosing cutter is constructed with a
rational tangent basis and rational bounds, avoiding rounded oblique cutter
faces. Both pieces share the operation budgets and undergo the same conversion
and manifold validation. Tangent planes return the unchanged solid on its
occupied side. Failures return neither partial piece. Tests include an analytic
oblique concave-prism volume integral, an obliquely split cavity, and a separately
computed convex-cell oracle for a non-prismatic three-dimensional branching solid.

`CombineWithTopologyHistory` additionally returns one `EdgeAncestry` per output
edge. It reports the two adjacent output faces' input ancestry and every
positive-length overlap with original topological edges in either operand,
including coincident inputs that do not own an output face. Exact rational
parameters identify the overlap interval along both output and input edges;
input parameters may decrease when orientation is reversed. Comparisons use
exact assembly coordinates before conversion, so rounded points cannot create
or erase ancestry. `HasGeneratedPortion` means these original-edge intervals do
not cover the whole output edge. Generated portions include Boolean intersection
edges and triangulation/conforming-subdivision edges; the flag does not imply a
new geometric crease. Vertex-only contacts are not edge ancestry.

This opt-in pass uses a separate `MaxWork` budget and checks every output edge
against every original topological edge. Ordinary Boolean and face-history APIs
retain their previous behavior. Errors return no partial topology history. IDs
are local to one operation and are not persistent CAD naming.
