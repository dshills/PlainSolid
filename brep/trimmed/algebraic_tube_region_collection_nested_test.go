package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/internal/testgraphfixture"
	"plainsolid/intersect"
	"sort"
	"testing"
)

func exactTripleRadialRegionFixture(t *testing.T) AlgebraicTubeRegions {
	t.Helper()
	ctx := context.Background()
	radii := testgraphfixture.TripleRadialRadii()
	// Author the physical degree6 polynomial directly as exact homogeneous
	// controls. No degree8 float chart is silently replaced or approximated.
	power := map[[2]int]*big.Rat{{0, 0}: big.NewRat(-1225, 256)}
	for _, r := range radii {
		factor := map[[2]int]*big.Rat{{0, 0}: new(big.Rat).Sub(big.NewRat(1, 2), new(big.Rat).Mul(r, r)), {1, 0}: big.NewRat(-1, 1), {0, 1}: big.NewRat(-1, 1), {2, 0}: big.NewRat(1, 1), {0, 2}: big.NewRat(1, 1)}
		next := map[[2]int]*big.Rat{}
		for a, x := range power {
			for b, y := range factor {
				k := [2]int{a[0] + b[0], a[1] + b[1]}
				if next[k] == nil {
					next[k] = new(big.Rat)
				}
				next[k].Add(next[k], new(big.Rat).Mul(x, y))
			}
		}
		power = next
	}
	var controls [][4]*big.Rat
	for i := 0; i <= 6; i++ {
		for j := 0; j <= 6; j++ {
			z := new(big.Rat)
			for k, c := range power {
				if k[0] > i || k[1] > j {
					continue
				}
				x := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k[0])), new(big.Int).Binomial(6, int64(k[0])))
				y := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(k[1])), new(big.Int).Binomial(6, int64(k[1])))
				z.Add(z, new(big.Rat).Mul(new(big.Rat).Mul(c, x), y))
			}
			controls = append(controls, [4]*big.Rat{big.NewRat(int64(i), 6), big.NewRat(int64(j), 6), z, big.NewRat(1, 1)})
		}
	}
	patch, e := geom.NewExactHomogeneousBezierSurface(ctx, 6, 6, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	roof, e := geom.NewExactBezierSurface(patch)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	vertices := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	var loop []Use
	for i, p := range vertices {
		loop = append(loop, Use{Edge: i, UVStart: p, UVEnd: vertices[(i+1)%4]})
	}
	patches := [2]intersect.Patch{{Surface: plane, Max: vertices[2]}, {Surface: roof, Max: vertices[2]}}
	faces := [2]Face{{Surface: plane, Outer: loop}, {Surface: roof, Outer: loop}}
	seeds, e := intersect.DiscoverRadialGraphContourTubes(ctx, patches[0], patches[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, radii, geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{})
	if e != nil {
		t.Fatal("original exact degree6 proposals", e)
	}
	t.Log("degree6 original proposals", len(seeds.Attempts))
	options := nestedTubeSectionOptions()
	options.MaxSegments = 512
	options.Trace.Coverage.OverlappingAffineXYFallback = true
	section, e := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, options)
	if e != nil || !section.Complete || len(section.Branches) != 3 {
		t.Fatal("complete exact three-loop section", len(section.Branches), e)
	}
	t.Log("degree6 complete three-loop section")
	regions, e := section.SealAlgebraicTubeRegions(ctx, nestedTubeRegionOptions())
	if e != nil || len(regions.regions) != 3 {
		t.Fatal("exact nested regions", e)
	}
	t.Log("degree6 nested containment proved")
	for i := range regions.regions {
		f := regions.RegionFaces(i)
		if f[1].SourceSurface != roof {
			t.Fatal("lost exact original roof")
		}
	}
	return regions
}

func TestCertifiedSelectedNestedDiskAnnulus(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("complete exact three-contour source and nested component fill")
	}
	r := exactTripleRadialRegionFixture(t)
	var selected []int
	for i, x := range r.regions {
		if len(x.Holes) == 0 || r.loops[x.Outer].Parent == -1 {
			selected = append(selected, i)
		}
	}
	sort.Ints(selected)
	if len(selected) != 2 {
		t.Fatal("disk plus nonadjacent annulus selection", selected)
	}
	for _, i := range selected {
		if r.regions[i].UpperChart != 1 {
			t.Fatal("original above-plane material side")
		}
	}
	o := AlgebraicTubeRegionCollectionMeshOptions{Component: AlgebraicTubeRegionMeshOptions{Footprint: AlgebraicTubeFootprintMeshOptions{Boundary: AlgebraicTubeRegionsBoundaryOptions{MaxOperations: 512000000, Loop: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .03, MaxOperations: 64000000}, MaxPieces: 512}, MaxOwnerOperations: 64000000}}, Polygon: trimvalidate.Options{MaxEdges: 2048, MaxWork: 8000000}}, Surface: MultiSpanTubeLensMeshOptions{MaxResidualOperations: 512000000, Closed: AlgebraicTubeLensMeshOptions{Collision: collision.Options{MaxPairs: 32000000}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .001}}, Span: MultiSpanTriangleBoundOptions{MaxDegree: 6, MaxResidualOperations: 16000000}}}}
	o.Component.Footprint.Boundary.Loop.Run.Partition.MaxOperations = 64000000
	if out, e := r.TessellateSelectedRegionsClosedCertified(context.Background(), []int{0, 1, 2}, o); e == nil || len(out.Mesh.Vertices) != 0 {
		t.Fatal("shared zero-thickness rims accepted", e)
	}
	out, e := r.TessellateSelectedRegionsClosedCertified(context.Background(), selected, o)
	if e != nil {
		t.Fatal("selected disk plus annulus", e)
	}
	if len(out.Components) != 2 || !out.Collision.Complete || len(out.Collision.Contacts) != 0 {
		t.Fatal("complete disjoint native display", out.Collision)
	}
	euler := 0
	covered := map[int]bool{}
	for i, c := range out.Components {
		edges := map[[2]int]bool{}
		for _, tri := range c.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}] = true
			}
		}
		chi := len(c.Mesh.Vertices) - len(edges) + len(c.Mesh.Triangles)
		want := 2 - 2*len(r.regions[selected[i]].Holes)
		if chi != want {
			t.Fatal("component genus", chi, want)
		}
		euler += chi
		for local, global := range out.ComponentVertices[i] {
			if covered[global] || out.Mesh.Vertices[global] != c.Mesh.Vertices[local] {
				t.Fatal("distinct component source incidence")
			}
			covered[global] = true
		}
	}
	if euler != 2 || len(covered) != len(out.Mesh.Vertices) {
		t.Fatal("disk/annulus aggregate incidence", euler, len(covered), len(out.Mesh.Vertices))
	}
	volume, _ := out.SignedDisplayVolume.Float64()
	want := tripleRadialAnalyticVolume()
	if math.Abs(volume-want) > .25*want {
		t.Fatal("independent original radial volume check", volume, want)
	}
	t.Log("certified disjoint disk/annulus display", len(out.Mesh.Vertices), len(out.Mesh.Triangles), "volume", volume, "analytic", want, "residual ops", out.ResidualOperations, "collision", out.CollisionPairs)
}
