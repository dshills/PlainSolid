# Paired circle parameter reversals

The exact two-/four-cell cylinder readers now accept an additional direct-model
encoding: a CIRCLE support normal of negative local Z, reference positive local
X, and EDGE_CURVE.same_sense false. The cylinder and cap support frames remain
unchanged. Reversing the circle normal reverses increasing physical azimuth;
false same_sense reverses traversal of that model parameter again. Consequently
the stored edge still advances positively between its exact cardinal endpoints,
so the existing angular role, coedge and closure proof applies unchanged.

Both original positive-normal/true-sense and negative-normal/false-sense pairs
are admitted independently per circle edge. Either reversal alone is rejected;
for quarter cells it selects the complementary long arc, and for half cells it
selects the other semicircle. Unpaired reversed axial LINE edges remain unsupported; the separately verified
paired encoding is described in CYLINDER_CELL_BOUND_ORIENTATION.md.
Arbitrary reference-phase changes and pcurve wrappers are still excluded.

Writers emit the original positive-normal/true-sense canonical encoding.
Reading alternate source flags preserves edge identity and material geometry,
not the exact source lexical encoding on later export. No global `analyticEdge`
helper or generic STEP route has been broadened.

The retained hand-authored alternate fixtures are independently loaded by OCCT:
four/six valid faces, volume12π, area20π and centroid(0,0,1.5). Tests prove exact
acceptance and atomic rejection of an individually changed sense, normal,
reference or axial-line sense. Set PLAINSOLID_OCCT_SPLIT_CYLINDER for the live
qualification alongside the existing rooted/model checks.
