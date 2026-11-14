package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedObliqueTorusCavityDisplay(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(1, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(1, 8)}}
	volume := 768*math.Pi - 6*math.Pi*math.Pi
	area := 320*math.Pi + 12*math.Pi*math.Pi
	previousV, previousA := math.Inf(1), math.Inf(1)
	for _, n := range []int{4, 8} {
		m, e := TessellateObliqueTorusCavityCertified(ctx, 3, 1, 8, -6, 6, pose, 1e-9, ObliqueTorusCavityMeshOptions{Deflection: 32 / float64(n*n), Subdivisions: n})
		if e != nil {
			t.Fatal(n, e)
		}
		if !m.BothDirections || !m.Collision.Complete || len(m.Collision.Contacts) != 0 || len(m.TorusEdges) != 32 || len(m.Mesh.Triangles) != 32*n*n+16*n {
			t.Fatal("certificate")
		}
		v, _ := m.SignedDisplayVolume.Float64()
		a := 0.
		shellVolume := [2]float64{}
		for i, t := range m.Mesh.Triangles {
			x, y, z := m.Mesh.Vertices[t[0]], m.Mesh.Vertices[t[1]], m.Mesh.Vertices[t[2]]
			a += y.Sub(x).Cross(z.Sub(x)).Length() / 2
			shell := 0
			if m.OriginalFace[i] >= 3 {
				shell = 1
			}
			shellVolume[shell] += x.Dot(y.Cross(z)) / 6
		}
		if shellVolume[0] <= 0 || shellVolume[1] >= 0 {
			t.Fatal("shell orientation", shellVolume)
		}
		ev, ea := math.Abs(v-volume), math.Abs(a-area)
		if ev >= previousV || ea >= previousA {
			t.Fatal("independent convergence", ev, ea, previousV, previousA)
		}
		previousV, previousA = ev, ea
		if n == 8 && (ev > volume*.02 || ea > area*.02) {
			t.Fatal("independent area/volume", v, a, volume, area)
		}
		for _, row := range m.TorusEdges {
			if len(row) != n+1 {
				t.Fatal("source edge samples")
			}
		}
	}
	for _, o := range []ObliqueTorusCavityMeshOptions{{Deflection: .01, Subdivisions: 1}, {Deflection: 2, Subdivisions: 4, MaxTriangles: 1}, {Deflection: 2, Subdivisions: 4, Evaluation: geom.SurfacePatchEvaluationOptions{MaxOperations: 1}}} {
		m, e := TessellateObliqueTorusCavityCertified(ctx, 3, 1, 8, -6, 6, pose, 1e-9, o)
		if e == nil || len(m.Mesh.Vertices) != 0 {
			t.Fatal("atomic budget/deflection")
		}
	}
}
