package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"reflect"
	"testing"
)

func TestPreparedStoredMixedDisplayPlacement(t *testing.T) {
	ctx := context.Background()
	displayOptions := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: collision.DefaultMaxPairs}}
	r := big.NewRat
	for _, lens := range []bool{false, true} {
		name := "warped"
		if lens {
			name = "lens"
		}
		t.Run(name, func(t *testing.T) {
			s := testWarpedSplinePrism(t, false, false)
			if lens {
				s = solidEllipticalLensFixture(t)
			}
			var raw bytes.Buffer
			if e := WriteJSON(&raw, s); e != nil {
				t.Fatal(e)
			}
			source, e := ReadJSON(bytes.NewReader(raw.Bytes()), JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(source.faces))}
			for i := range o.Faces {
				o.Faces[i] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}}}
			}
			prepared, e := source.PrepareMixedClosedDisplay(ctx, o, displayOptions)
			if e != nil {
				t.Fatal(lens, e)
			}
			originalBound := prepared.SourceSquaredDistance()
			if originalBound.Sign() <= 0 {
				t.Fatal("actual nonzero curved source bound")
			}
			pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
			if e != nil {
				t.Fatal(e)
			}
			world, e := prepared.Place(ctx, pose, displayOptions)
			if e != nil {
				t.Fatal(e)
			}
			if world.SquaredDistance().Cmp(originalBound) < 0 || len(world.Display().Shells()) != 1 || world.Display().Shells()[0].Depth != 0 {
				t.Fatal("placed proof")
			}
			ancestry := world.Ancestry()
			if len(ancestry.EdgeVertices) != len(source.edges) || len(ancestry.FaceVertices) != len(source.faces) {
				t.Fatal("source ancestry")
			}
			ids := world.Display().Faces()
			m := world.Display().Mesh()
			for tri, face := range ids {
				if face < 0 || face >= len(source.faces) {
					t.Fatal("face owner")
				}
				allowed := map[int]bool{}
				for _, id := range ancestry.FaceVertices[face] {
					allowed[id] = true
				}
				for _, id := range m.Triangles[tri] {
					if !allowed[id] {
						t.Fatal("triangle lost source owner")
					}
				}
			}
			for edge, row := range ancestry.EdgeVertices {
				if len(row) != len(ancestry.EdgeParameters[edge]) || len(row) < 2 {
					t.Fatal("edge partition")
				}
				for i := 1; i < len(row); i++ {
					if ancestry.EdgeParameters[edge][i].Cmp(ancestry.EdgeParameters[edge][i-1]) <= 0 {
						t.Fatal("original parameter order")
					}
				}
			}
			expected, tolerance := 5., .05
			if lens {
				expected = math.Pi / 1024
				tolerance = expected * .15
			}
			if math.Abs(m.SignedVolume()-expected) > tolerance {
				t.Fatal("independent analytic source volume")
			}
			// The original source snapshot is the same native entity graph after reload.
			var provenance bytes.Buffer
			if e := WriteJSON(&provenance, world.Source()); e != nil || !bytes.Equal(raw.Bytes(), provenance.Bytes()) {
				t.Fatal("source binding", e)
			}
			originalBound.SetInt64(999)
			world.SquaredDistance().SetInt64(999)
			ancestry.EdgeParameters[0][0].SetInt64(999)
			ancestry.EdgeVertices[0][0] = 999
			if prepared.SourceSquaredDistance().Cmp(r(999, 1)) == 0 || world.SquaredDistance().Cmp(r(999, 1)) == 0 || world.Ancestry().EdgeParameters[0][0].Cmp(r(999, 1)) == 0 || world.Ancestry().EdgeVertices[0][0] == 999 {
				t.Fatal("public alias")
			}
			second, e := prepared.Place(ctx, pose, displayOptions)
			if e != nil || !reflect.DeepEqual(second.Display().Mesh(), m) || second.SquaredDistance().Cmp(world.SquaredDistance()) != 0 {
				t.Fatal("reused proof", e)
			}
			if denied, e := prepared.Place(ctx, pose, placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}); e == nil || denied != nil {
				t.Fatal("atomic world limit")
			}
			limited := o
			limited.MaxTriangles = 1
			if denied, e := source.PrepareMixedClosedDisplay(ctx, limited, displayOptions); e == nil || denied != nil {
				t.Fatal("cannot wrap unproved source")
			}
		})
	}
}
