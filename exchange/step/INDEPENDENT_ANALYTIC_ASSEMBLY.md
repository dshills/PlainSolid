# Flat planar / cylinder / torus product assembly

`ReadIndependentAnalyticAssembly`, `WriteIndependentAnalyticAssembly` and
`WriteIndependentAnalyticAssemblySource` add a distinct three-family workflow.
There is exactly one assembly-only root and three separately owned local products:
one cardinal planar material, one finite model-only split cylinder, and one
model-only natural ring torus. The torus is used at least twice, the other leaves
at least once, with four to sixteen flat proper signed-coordinate occurrences.
Products retain one common explicit metre or millimetre unit. No nested groups,
implicit placements, Boolean merging or cross-product geometry sharing is added.

The cylinder uses the existing strict two-half/four-quarter material proof:
original positive radius and finite height, analytic cylindrical wall supports,
two planar caps, direct LINE/CIRCLE model edges and their complete directed
incidence. Its native circular rational patches and rim ancestry are regenerated
by the existing exact constructor. No vendor-rounded pcurve support is introduced.
The torus and planar proofs are unchanged. Each local product is independently
proved once; cumulative native topology and original parse limits still apply.
Exact rational from/to frame composition must remain representable as the proper
signed-coordinate occurrence matrix. All original definition, representation,
body, occurrence and source face/edge roles remain available.

The older `IndependentTorusAssembly` APIs still require exactly the planar/torus
pair and reject this three-family graph. Their native version-1 schema and bytes
are unchanged. The new `ReadIndependentAnalyticAssemblyJSON` and
`WriteIndependentAnalyticAssemblyJSON` use the separate version-1 tag
`plainsolid-step-independent-analytic-assembly`. The cylinder member claims
explicitly include its local frame and original/native rim roles. Source-bound
reload performs one complete strict graph proof and compares all stored native
geometry and metadata; native JSON is not trusted as input geometry. The same
16 MiB envelope ceiling and separate source limits/cancellation rules apply.

`tools/occtref/author_step_independent_analytic_assembly.py` independently authors
an AP214 product graph from the retained genuine OCCT box and separately authored
exact torus and quarter-cell cylinder geometry. No Go output or rounded vendor
curve repair is used. The cylinder is rotated from local +Z onto world +X and
translated by (0,-20,10). The unit context explicitly contains length, radians
and steradians, as required by the existing strict cylinder parser.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`, using the already qualified
`step_independent_torus_assembly` comparator, accepts original source, Go-authored
output and source re-export after native reload through ordinary TransferRoots:

- Four solid occurrences, fourteen faces and 56 edge uses.
- Volume `24+12*pi²+12*pi`, area `52+24*pi²+20*pi` in mm.
- World bounds `[-10,-22,-2]` to `[25,25,12]`, allowing the documented OCCT
  numerical topology margin (2e-7 comparison threshold).
- Surface centroid independently calculated from the box, two placed toroids
  and placed cylinder agrees with OCCT surface integration.

Set `PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY` to that comparator binary to run
live checks. Tests also preserve the two-half cylinder alternative, old/new API
and tag separation, canonical native bytes, exact source ancestry, native/source
mutation rejection, geometry borrowing/duplicate families, invalid units/senses,
missing leaves, cycles, limits/cancellation and atomic failure. STEP and native
JSON fuzz targets use fixed source and native topology budgets.
