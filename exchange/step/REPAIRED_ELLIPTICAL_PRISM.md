# Authenticated elliptical prism shell with explicit seam repair

`RepairRoundedEllipticalPrismShell` extends the separate elliptical extrusion
repair to an original closed OCCT prism: one `MANIFOLD_SOLID_BREP`, one
`CLOSED_SHELL`, three `ADVANCED_FACE`s, two vertices and three model edges.
This API returns a three-face shell descriptor, **not `trimmed.Solid`**. It does
not bypass native solid assembly/meshing requirements or enable generic STEP
solid import. Existing strict readers remain unchanged.

The pinned source is authored with `BRepPrimAPI_MakePrism` from a planar ellipse
disk, rather than by modifying Go output. Its two natural ellipse rim edges
are shared by the lateral face and their disk caps. Each rim `SURFACE_CURVE`
has one extrusion LINE pcurve and one planar ELLIPSE pcurve. The seam has two
independent lateral pcurves and opposite coedge uses. Its second U coordinate
is still decimal 6.28318530718; original source closure is not mathematically
exact. The explicit caller tolerance and displacement certificate from
[the side-face repair](REPAIRED_ELLIPSE_EXTRUSION.md) remain mandatory.

## Complete graph and continuous geometry checks

A bounded structural discovery identifies all 121 source roles, independently
of record IDs. This discovery is not acceptance. The lateral subgraph is
projected into the already qualified 89-record side-face proof, normalizing
only the topological wrapper, cap owner lists and standalone orientation.
Every scalar admitted by that proof is then placed into the complete expected
closed graph; the final full graph comparison is exact and bijective.

Additional cap data is not copied unchecked: cap plane origin, axis and
reference must equal the corresponding natural model rim frame; UV ellipse
radii must equal original model radii, with zero UV origin and positive U
reference. Thus plane(a cos t,b sin t) equals the original model rim at every
t, while the side's endpoint V has the exact same rim. No finite samples are
used to accept these identities. All other numeric values, flags, references
and multiplicities are checked against the bounded original syntax.

The source side face and lower cap have reversed face and bound senses; the
upper cap has positive senses. Native UV outer loops are positive, with the
corresponding `Face.Reversed` flags. Opposite effective rim orientations and
the two seam uses are preserved. The repaired side and disks form the intended
analytic ellipse-prism boundary. This milestone does not construct a native
material `Solid` or claim a generic embedding algorithm.

`Source*IDs` retain original body, shell, face, model edge, seam pcurve and
per-rim owner pcurve IDs. `Side.Source` is explicitly a normalized independently
validated proof partition with local IDs, not the original closed source.
`WriteEllipticalPrismShellSource` re-proves the whole original graph before
emitting it, retaining the original rounded phase. It is source re-export,
not a repaired STEP serialization. Individual native analytic supports are
losslessly serializable through the existing native surface codec.

Input is bounded to 64 KiB, 121 entities, 4096 values and depth16, respecting
smaller caller limits. No partial result is returned after any failure. Source
writer bounds caller-owned trees before serialization and validates before
writing bytes.

## Independent qualification

Pinned OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9`:

```sh
sh tools/occtref/build_step_elliptical_prism.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/elliptical-prism
/tmp/elliptical-prism write /tmp/prism.step
PLAINSOLID_OCCT_ELLIPTICAL_PRISM=/tmp/elliptical-prism go test ./exchange/step -run TestOCCTRepairEllipticalPrism -v
```

Both original and source re-export pass conventional TransferRoots and
BRepCheck: one solid, three faces, six edge uses (three unique model edges).
Ellipse radii4/2, height3, original pose center(1,2,3), axis+Y yield:

- Area108.39617178072274 from independent OCCT surface derivative quadrature
  plus exact planar cap integration, matching 3×ellipse perimeter+16π.
- Volume75.398223686155035 from independently integrated oriented surface flux,
  also matching OCCT BRepGProp volume and analytic24π.
- Surface centroid(1,3.5,3), matching symmetry.

Numerical oracle values validate interoperability but do not replace exact
source acceptance. Mutations reject duplicate cap ownership, switched rim,
foreign pcurve owner, wrong cap frame/radius/sense, bound or seam reversal and
excessive seam phase. The previous authentic side-face source remains separate.
