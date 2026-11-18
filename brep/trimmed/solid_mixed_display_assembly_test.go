package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func prepareAssemblySource(t *testing.T, lens bool, deflection float64) *PreparedStoredMixedDisplay {
	t.Helper()
	s := testWarpedSplinePrism(t, false, false)
	if lens {
		s = solidEllipticalLensFixture(t)
	}
	o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(s.faces))}
	for i := range o.Faces {
		o.Faces[i] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: deflection}}}}
	}
	p, e := s.PrepareMixedClosedDisplay(context.Background(), o, placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1000000}})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func assemblyOccurrence(t *testing.T, p *PreparedStoredMixedDisplay, x, y, z *big.Rat, rotate bool) *PlacedStoredMixedDisplay {
	t.Helper()
	r := big.NewRat
	m := [3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), x}, {r(0, 1), r(1, 1), r(0, 1), y}, {r(0, 1), r(0, 1), r(1, 1), z}}
	if rotate {
		m[0][0], m[0][2], m[2][0], m[2][2] = r(3, 5), r(4, 5), r(-4, 5), r(3, 5)
	}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	out, e := p.Place(context.Background(), pose, placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1000000}})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestMixedDisplayAssemblyIndependentSources(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	prism, lens := prepareAssemblySource(t, false, .02), prepareAssemblySource(t, true, .02)
	assets := []*PlacedStoredMixedDisplay{assemblyOccurrence(t, prism, r(1, 10), r(0, 1), r(0, 1), false), assemblyOccurrence(t, lens, r(10, 1), r(-1, 5), r(3, 8), true)}
	collection, e := AssemblePlacedMixedDisplays(ctx, assets, MixedDisplayAssemblyOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 2000000}})
	if e != nil {
		t.Fatal(e)
	}
	assets[0] = nil
	if len(collection.Shells()) != 2 || math.Abs(collection.Mesh().SignedVolume()-(5+math.Pi/1024)) > .051 {
		t.Fatal("independent collection volume")
	}
	if collection.SquaredDistance().Sign() <= 0 {
		t.Fatal("lost real curved error")
	}
	owners := collection.TriangleOwners()
	m := collection.Mesh()
	offset := 0
	triOffset := 0
	for i := 0; i < 2; i++ {
		asset, ok := collection.Asset(i)
		if !ok {
			t.Fatal("missing original asset")
		}
		ancestry, ok := collection.Ancestry(i)
		if !ok {
			t.Fatal("ancestry")
		}
		local := asset.Display().Mesh()
		faces := asset.Display().Faces()
		original := asset.Ancestry()
		for j, v := range local.Vertices {
			if m.Vertices[offset+j] != v {
				t.Fatal("changed certified point")
			}
		}
		for j, tr := range local.Triangles {
			if owners[triOffset+j] != (MixedDisplayTriangleOwner{i, faces[j]}) {
				t.Fatal("asset/face identity")
			}
			for k, id := range tr {
				if m.Triangles[triOffset+j][k] != offset+id {
					t.Fatal("topological welding")
				}
			}
		}
		for edge, row := range original.EdgeVertices {
			if !reflect.DeepEqual(original.EdgeParameters[edge], ancestry.EdgeParameters[edge]) {
				t.Fatal("changed exact model parameter")
			}
			for j, id := range row {
				if ancestry.EdgeVertices[edge][j] != id+offset {
					t.Fatal("native seam identity")
				}
			}
		}
		if collection.SquaredDistance().Cmp(asset.SquaredDistance()) < 0 {
			t.Fatal("aggregate source bound")
		}
		offset += len(local.Vertices)
		triOffset += len(local.Triangles)
	}
	// This lens is strictly within the prism's analytic XY footprint and
	// between its floor and roof. A disjoint collection must not reinterpret it
	// as a cavity or Boolean nesting relation.
	nested := assemblyOccurrence(t, lens, r(0, 1), r(0, 1), r(-3, 2), false)
	outer, _ := collection.Asset(0)
	if got, e := AssemblePlacedMixedDisplays(ctx, []*PlacedStoredMixedDisplay{outer, nested}, MixedDisplayAssemblyOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 2000000}}); !errors.Is(e, geom.ErrConvergence) || got != nil {
		t.Fatal("nested independent source accepted", e)
	}
	proof := collection.Separations()
	if len(proof) != 1 || new(big.Rat).Mul(proof[0].DisplayGap, proof[0].DisplayGap).Cmp(proof[0].RequiredGapSquared) <= 0 {
		t.Fatal("strict source image separation")
	}
	proof[0].DisplayGap.SetInt64(-99)
	owners[0].Asset = 99
	m.Vertices[0].X = 99
	collection.SquaredDistance().SetInt64(-99)
	if collection.Separations()[0].DisplayGap.Sign() <= 0 || collection.TriangleOwners()[0].Asset == 99 || collection.Mesh().Vertices[0].X == 99 || collection.SquaredDistance().Sign() < 0 {
		t.Fatal("mutable certificate")
	}
}
func TestMixedDisplayAssemblyGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	p := prepareAssemblySource(t, false, .04)
	base := assemblyOccurrence(t, p, r(0, 1), r(0, 1), r(0, 1), false)
	far := assemblyOccurrence(t, p, r(10, 1), r(0, 1), r(0, 1), false)
	inputs := []*PlacedStoredMixedDisplay{base, far}
	t.Logf("aggregate triangles=%d", len(base.Display().Mesh().Triangles)+len(far.Display().Mesh().Triangles))
	if _, e := AssemblePlacedMixedDisplays(ctx, inputs, MixedDisplayAssemblyOptions{Validation: collision.LocationOptions{MaxTriangles: collision.MaxTriangles, MaxCollisionPairs: 2000000, MaxWork: 10000000}}); e != nil {
		t.Fatal(e)
	}
	// Original displays are separated by 1e-4, but their nonzero actual source
	// deviations do not prove separation. Exact envelope admission must fail.
	close := assemblyOccurrence(t, p, r(20001, 10000), r(0, 1), r(0, 1), false)
	touch := assemblyOccurrence(t, p, r(2, 1), r(0, 1), r(0, 1), false)
	for _, other := range []*PlacedStoredMixedDisplay{base, close, touch} {
		if got, e := AssemblePlacedMixedDisplays(ctx, []*PlacedStoredMixedDisplay{base, other}, MixedDisplayAssemblyOptions{}); !errors.Is(e, geom.ErrConvergence) || got != nil {
			t.Fatal("uncertain/overlapping/contact accepted", e)
		}
	}
	for _, o := range []MixedDisplayAssemblyOptions{{MaxAssets: 1}, {MaxVertices: 1}, {MaxTriangles: 1}, {Validation: collision.LocationOptions{MaxCollisionPairs: 1}}, {Validation: collision.LocationOptions{MaxWork: 1}}} {
		if got, e := AssemblePlacedMixedDisplays(ctx, inputs, o); e == nil || got != nil {
			t.Fatal("atomic quota")
		}
	}
	if got, e := AssemblePlacedMixedDisplays(ctx, []*PlacedStoredMixedDisplay{base, far, far}, MixedDisplayAssemblyOptions{MaxAssetPairs: 1}); e == nil || got != nil {
		t.Fatal("pair preflight")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := AssemblePlacedMixedDisplays(canceled, inputs, MixedDisplayAssemblyOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
