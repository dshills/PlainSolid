# Strict model-only elliptical prism import

`ReadModelOnlyEllipticalPrism` directly accepts the separately qualified
76-record native STEP dialect and constructs an exact `trimmed.Solid`.
It does not invoke the rounded-seam repair reader, create pcurves, choose a
nearby period, compare geometry to a repair tolerance, or use OCCT at runtime.

The entire product/body/shell/face/edge graph is injectively matched, permitting
arbitrary original IDs and record ordering while rejecting detached/cross-owned
records. Structural role discovery is only preliminary. The final proof checks
all source numbers and references after deriving the exact source geometry:

- signed-cardinal source frame and strictly positive ellipse radii a>b;
- positive exact height from the cap center and profile axis;
- two natural full model ellipse rims with matching profile axes and radii;
- shared LINE through their exact +major vertices, in positive physical height;
- original negative unit extrusion axis, with the finite side selected between
  the two complete natural rim edges;
- two occurrences of the same seam with opposite coedge directions;
- cap planes exactly coincident with rim frames and opposite material senses;
- full closed-shell ownership and the registered millimeter/radian unit context.

Full rims are natural closed ELLIPSE edges with the same start/end vertex. No
finite angular trim or decimal periodic parameter is present. The bounded
native authoring dialect selects the complete natural ellipse once and the
finite lateral band joined by its single seam. Other windings, sense variants,
noncardinal frames, external pcurves, rounded source graphs and unknown roots
are not admitted. Native tolerance applies to implementation validation only;
it never changes a coordinate or weakens any exact source predicate.

The reader shares only the immutable quarter construction with the repaired
material path: weights(1,1,2), four rational tensor walls and two planar caps.
All native poles must fit binary64 exactly. Exact sewing proves continuous
edge identity, while positive disjoint quadrants and height give the same
constructive material embedding. Native partition ancestry retains original
full rim and face IDs. Zero-translation write→strict read produces exactly the
same native serialized control bases/topology as the existing promoted solid.

`WriteModelOnlyEllipticalPrismJSON` and `ReadModelOnlyEllipticalPrismJSON` use a
separate version1 envelope with source, native payload and ancestry. There is
**no repair field**. They regenerate native material and reject stale geometry,
ancestry or forged payload atomically. The repaired material format and exact
format cross-reject. `WriteModelOnlyEllipticalPrismSource` re-proves and retains
this exact source. Original repaired-source APIs are unchanged.

Input limits:64KiB,76entities,4096values,depth16 (smaller caller limits apply).
Native envelopes are bounded to512KiB. Context cancellation and unrepresentable
control points fail without partial native results.

Tests cover generated native STEP→strict import→ordinary trimmed JSON and
exact provenance JSON→source STEP; translated/name preservation; continuous
zero-error edge certificates; mass; malformed owner/radius/cap/frame/sense/unit
mutations; arbitrary ID rebasing/reversed record ordering; bounds; native
payload tamper and cross-dialect rejection. The optional pinned OCCT helper
independently validates the translated exact chain (one solid, three external
faces, volume24π, area108.39617178072274, center(9,1.5,7)). Pure Go tests require
no external executable.

```sh
PLAINSOLID_OCCT_ELLIPTICAL_PRISM=/tmp/plainsolid-step-elliptical-prism \
  go test ./exchange/step -run TestOCCTReadModelOnlyEllipticalPrism -v
```
