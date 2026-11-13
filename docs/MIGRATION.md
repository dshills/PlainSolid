# Moving a modeling workflow from OCCT to PlainSolid

PlainSolid uses Go values and immutable shape constructors. Its compatibility
target is behavior, not the OCCT C++ API or ABI. Start by matching the
geometric operation and its input requirements in the [status matrix](STATUS.md).
If an operation is outside the implemented domain, keep using OCCT for that
operation and exchange the geometry through a format both kernels support.

| OCCT workflow | PlainSolid starting point |
|---|---|
| `gp_Pnt`, `gp_Vec`, `gp_Ax2`, `gp_Trsf` | `geom.Vec3`, `geom.Frame`, `geom.Affine`. Model coordinates are unitless; angles are radians. |
| `Geom_BSplineCurve`, `Geom_BSplineSurface` | `geom.NewBSpline`, `geom.NewBSplineSurface`. Expand knot multiplicities into the knot vector. Surface poles are indexed `u*CountV+v`. |
| Periodic spline surface or surface fitting | `geom.NewWrappedSurfacePatch` checks exact seam continuity on a fundamental patch. `ApproximateScatteredBSplineSurface` fits fixed-basis splines to parameterized samples outside tensor grids. |
| `BRepPrimAPI_MakeBox` or a planar polygon prism | `brep.Box`, `brep.PrismWithHoles`. Coordinates are copied into an indexed face/edge graph. |
| Finite planar extrusion to a selected face | `brep.PrismToFace` verifies that the projected polygon with holes is covered by the selected polygonal face, including oblique stops. `trimmed.CylinderToFace` and `CylinderToObliqueFace` cover axis-aligned circular sweeps against supported polygon faces. `CylinderToRoundFace` and `PrismToRoundFace` recognize supported round caps from the actual trimmed boundary. Reevaluate the face reference after upstream edits. |
| Signed circular extrusion with eccentric circular holes | `trimmed.ExtrudeDiskWithHoles` retains round stock, through bores, and blind-pocket floors. `ExtrudeDiskWithHolesCertified` provides a closed mesh with original face IDs; `step.WriteDiskHoleExtrusions` exports the analytic boundary and `ReadDiskHoleExtrusions` checks canonical reimports. |
| Mixed line and quarter-circle sketch | `trimmed.ExtrudeRoundedRectangle` supports independent corner radii and circular bores or pockets. Use `ExtrudeRoundedRectangleCertified` for mesh ancestry and `step.WriteRoundedRectangleExtrusions` and `ReadRoundedRectangleExtrusions` for canonical STEP round trips. |
| Signed polygon extrusion with circular holes | `trimmed.ExtrudePolygonWithHoles` builds a convex outer profile with strictly interior, disjoint through bores and blind pockets with planar floors in the sketch frame. |
| `BRepPrimAPI_MakeCylinder`, sphere, cone or torus | `brep/parametric` analytic primitives; use `trimmed.FromParametric` if you also need face wires. |
| `BRepPrimAPI_MakeRevol` with a polygon meridian | `trimmed.RevolvePolygon` accepts signed partial sweeps and a full turn, with concave meridian caps and an optional contiguous axis side. |
| `BRepPrimAPI_MakeRevol` with an off-axis disk meridian | `trimmed.RevolveCircle` constructs a signed torus elbow or a full ring torus in the supplied frame. |
| Constant-lead helical sweep of a polygon | `trimmed.SweepHelicalPolygon` retains analytic thread flank supports, shared helical edges, and planar ends for a strictly positive-radius profile whose axial width is less than one lead. `HelicalCylinderCommon` restricts a convex sweep to a coaxial radial band when axial caps do not cut it. |
| Blind-ended helical grooves on a cylinder | `trimmed.GrooveCylinderWithHelicalStarts` cuts separated convex profiles that meet the original wall in one side, using a pitch-matched seam to keep multi-turn openings in one chart. |
| Blind-ended internal helical grooves | `trimmed.GrooveTubeWithHelicalStarts` cuts the inner wall of an annular cylinder and retains both annular caps. |
| Planar `BRepAlgoAPI_Fuse/Common/Cut` | `csg/poly.CombineWithHistory` for embedded planar solids, with explicit limits. Its result ancestry is local to the operation. |
| Spline graph or supported spline shell `BRepAlgoAPI_Fuse/Common/Cut` | `trimmed.CombineAlignedSplineGraphSolids` for a shared-base envelope, or `trimmed.CombineSplineShells` for certified rectangular and selected wrapped spline solids. `CombineSectionedSplineShells` accepts tube proposals for mixed open and closed arrangements, verifies the complete contact set, and maps retained faces and edge fragments. |
| Sewing or same-domain healing | `trimmed.Sew` for certified spline or selected analytic edge matches. `UnifyAdjacentSplineFaces` merges rectangles on an exactly identical spline chart; `MergeSplitSplineEdgeExact` removes eligible valence-two edge splits. `heal/unify.UnifyWithHistory` provides planar ancestry. |
| `BRepAlgoAPI_Section` or boundary imprint | `trimmed.SectionSplineShells` returns certified transverse wires; `SplitSectionedSplineShells` partitions both source boundaries without removing material. `SectionAffineSplineShell` returns exact planar section ancestry. |
| Algebraic spline seams and face fragmentation | `SubdivideAlgebraicSeams` retains exact roots in both coedges. `ArrangeFacesByAlgebraicUVLines` constructs bounded straight chart cuts with the original spline model curve; `SewAlgebraicArrangementFragments` verifies declared full edge matches and vertex links. |
| Section crossing original spline shell edges | `SectionSplineShellsAlgebraic` keeps exact edge parameters and original face ownership for supported rectangular charts. `SubdivideOriginalEdges` shares each proved split; `ImprintOriginalShells` arranges both sets of faces, and `CombineAlgebraic` returns a certified algebraic boundary for selected regular Booleans. |
| Split a selected closed spline lens with a vertical plane | `SplitRationalEllipseLensByVerticalPlane` returns two certified material halves with the original graph supports, exact cap boundaries and rational material queries. |
| Closed section lens on two coordinate-plane graphs | `SectionTrimmedSplinePair` and `SealProjectedAlgebraicTubeLens` retain original chart supports. Use `SectionProjectionPlane`, `ClipMaterialLine`, or `TessellateClosedCertified` in the original model coordinates. |
| `BRepFilletAPI_MakeFillet` on vertical edges of a convex prism | `csg/fillet.PrismCorners` selects independent radii by source profile corner. The document prism fillet recipe retains the selection across save and edit. |
| `BRepFilletAPI_MakeFillet` on the circular end rims of a cylinder | `csg/fillet.WrappedCylinderRims` builds exact quarter-toroid blends. A zero radius leaves an end sharp. |
| Complete offset of a circular cylinder or supported rounded cylinder | `csg/offset.WrappedRoundedCylinder` applies a signed Euclidean offset to the complete boundary, retaining face ancestry. |
| `BRepOffsetAPI_DraftAngle` on convex planar parts | `csg/draft.Shear` moves selected supporting planes around a neutral plane. For independent transforms use `draft.Apply`. |
| Two-distance planar edge chamfer | `csg/draft.Chamfer` on convex parts when the in-face Euclidean setbacks are exactly representable; it also offers a distinct support-coordinate mode. |
| Drill through a convex planar part | `trimmed.BoreCylinderAlong` or `BoreCylinders`. Supply entry and exit faces. Openings must clear all other faces. |
| Rational cylinder, draft and coaxial bore | Use `geom.NewCanonicalWrappedCylinder`, `trimmed.NewWrappedCylinder`, `draft.WrappedFrustum` and `trimmed.NewWrappedCylinderBore`. The document feature kinds preserve the exterior wall and cap roles across regeneration. |
| `BRepGProp` volume/area/center | `MassProperties` on the representation. Curved mass uses direct boundary quadrature with convergence observations, not tessellation. |
| `BRepMesh_IncrementalMesh` | `Tessellate`; set model-space deflection and triangle limits. Request an analytic bound where the surface/trim combination supports it. |
| Exact point location and projection | `LocateRationalOnSplineFace` combines exact projected inversion and UV winding. Wrapped cylinder and frustum queries support certified finite line contacts, section curves and distance bounds on their canonical material domain. |
| `STEPControl_Reader` | `exchange/step.Read` for planar solids, `ReadTrimmedBodies` for supported analytic graphs, `ReadSplineShells` for certified rectangular spline shells, and `ReadAffineSplineShells` for exact affine spline solids with supported trims. `ReadOrthogonalSplineOpenShells` retains connected open topology; `Preflight` inventories record types before reader selection. |
| `STEPCAFControl_Reader` | `step.ReadAssembly` and `document.FromSTEPAssembly` for named rigid instances; use face-color readers for the supported style graphs. |
| `IGESControl_Reader` | `exchange/iges.Read` supports documented planar and rational spline type 142/144 faces. `Model.OrthogonalBoundaries` exposes verified orthogonal loops and holes for supported rigid placements. |
| IGES closed spline lens and refined seam curves | `exchange/iges.ReadPartitionedRationalSplineLensSolid` retains exact model and per-owner curve bases, knot-cell intervals and the explicit source edge graph for the supported lens family. |
| OCAF document transactions | `document.Document`, snapshot transactions and typed labels. A feature recipe is a Go value with explicit dependencies. |
| `AIS_InteractiveContext` or `HLRBRep` | `scene` and `document.Presentation` for headless picking; `DrawVisibleEdges` and `scene.WriteSVG` for drawing against a mesh. |

Keep the exact representation for modeling and document storage. Build a mesh
when a display, mesh export, or mesh-only query needs one. Report parser limits,
Boolean errors, and unresolved numerical diagnostics to the caller; a partial
contact covering cannot be used to rule out intersections.

The [bracket example](../examples/bracket) demonstrates planar through-cut
and STL export. The [pressure cap example](../examples/pressurecap) writes
both exact native geometry and a mesh. The [bored plate example](../examples/boredplate)
retains a face-named machining recipe and writes a perspective drawing.
The [wrapped bore example](../examples/wrappedbore) writes the same blind
bore as an exact STEP solid and native trimmed JSON.

For a production migration, add a fixture for each modeling and exchange
workflow you depend on. Check properties and geometric invariants in both
kernels instead of comparing internal topology order. See the [reference
runner](../tools/occtref/README.md) to add an independent OCCT check.
