package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func algebraicCapFixture(t *testing.T, height *big.Rat) AlgebraicLensPlaneSplit {
	return algebraicCurvedCapFixture(t, height, 0)
}
func algebraicCurvedCapFixture(t *testing.T, height *big.Rat, bend float64) AlgebraicLensPlaneSplit {
	return algebraicCapSpansFixture(t, height, bend, false, false)
}
func algebraicCapSpansFixture(t *testing.T, height *big.Rat, bend float64, refine, crease bool) AlgebraicLensPlaneSplit {
	t.Helper()
	graph := func(z, f float64) *geom.BSplineSurface {
		term := []float64{.25, -.25, .25}
		var poles []geom.Vec3
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z + f*(1./16-term[i]-2*term[j]) + bend*([]float64{0, 0, 1}[i]/4+[]float64{0, 0, 1}[j]/8)})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}

		if refine {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			p, e = p.InsertUKnot(.5, 2, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			p, e = p.InsertVKnot(.5, 2, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			s, e = p.ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
			if crease {
				data := s.Data()
				for i, p := range data.Poles {
					data.Poles[i].Z += math.Abs(p.X-.5) / 8
				}
				s, e = geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		return s
	}
	hint := geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{big.NewRat(11, 16), big.NewRat(-1, 1), big.NewRat(1, 1), big.NewRat(-2, 1), big.NewRat(2, 1), new(big.Rat)}}
	s, e := SplitRationalEllipseLensByVerticalPlane(context.Background(), graph(0, 0), graph(2, 1), graph(2, 0), hint, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat).Neg(height)}, 1e-9, geom.GraphPointContactOptions{}, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCertifiedAlgebraicLensCap(t *testing.T) {
	ctx := context.Background()
	source := algebraicCapFixture(t, big.NewRat(9, 16))
	o := AlgebraicCapMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .00025}}
	originalVertices := source.Source.Vertices()
	var results []CertifiedAlgebraicCapMesh
	for _, side := range []int{-1, 1} {
		got, e := source.TessellateCapCertified(ctx, side, o)
		if e != nil {
			t.Fatal(e)
		}
		results = append(results, got)
		if len(got.Mesh.Triangles) != len(got.Mesh.Vertices)-2 {
			t.Fatal("disk coverage")
		}
		for owner, path := range got.CurveVertices {
			for i, id := range path {
				if got.Mesh.Vertices[id] != got.Curves[owner].Vertices[i].Point {
					t.Fatal("boundary samples moved")
				}
			}
		}
		for _, tri := range got.Mesh.Triangles {
			a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
			if b.Sub(a).Cross(c.Sub(a)).Y*float64(-side) <= 0 {
				t.Fatal("cap outward orientation")
			}
		}
		area := got.Mesh.Area()
		height := 7. / 128
		want := 4. / 3 * height * math.Sqrt(height)
		if math.Abs(area-want) > .0002 {
			t.Fatal("independent analytic cap area", area, want)
		}
		exact, _ := got.ProjectedArea.Float64()
		if math.Abs(exact-area) > 1e-14 {
			t.Fatal("exact planar coverage")
		}
		report, e := collision.SelfIntersections(got.Mesh, collision.Options{MaxPairs: 100000})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal("cap contacts", e)
		}
		t.Log("side", side, "triangles", len(got.Mesh.Triangles), "area", area, "analytic", want, "operations", got.ExactOperations)
	}
	if len(results[0].Mesh.Vertices) != len(results[1].Mesh.Vertices) {
		t.Fatal("halves disagree")
	}
	for i, p := range results[0].Mesh.Vertices {
		if p != results[1].Mesh.Vertices[i] {
			t.Fatal("cap seam rounding differs")
		}
	}
	for i, v := range source.Source.Vertices() {
		if v != originalVertices[i] {
			t.Fatal("source topology display mutated")
		}
	}

	// Mutating all exported recipe containers must not alter verified source.
	escaped := results[0].SourceCurves[0]
	escaped.Partition.Line[0].SetInt64(99)
	escaped.Partition.CutUV[0].Coordinates[0].Lower.SetInt64(99)
	escaped.Partition.Fragments[0][0].BoundaryEdge = -999
	if len(escaped.Model.Events) > 0 {
		escaped.Model.Events[0].Lower.SetInt64(99)
	}
	escaped.Model.Cells[0] = nil
	again, e := source.TessellateCapCertified(ctx, -1, o)
	if e != nil || again.ProjectedArea.Cmp(results[1].ProjectedArea) != 0 {
		t.Fatal("caller mutation reached source", e)
	}

	low := o
	low.Display.MaxOperations = 1
	if partial, e := source.TessellateCapCertified(ctx, -1, low); e == nil || len(partial.Mesh.Triangles) != 0 {
		t.Fatal("budget partial")
	}
	nonbinary := algebraicCapFixture(t, big.NewRat(11, 20))
	if partial, e := nonbinary.TessellateCapCertified(ctx, -1, o); !errors.Is(e, geom.ErrUnrepresentable) || len(partial.Mesh.Triangles) != 0 {
		t.Fatal("nonplanar rounded plane accepted", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := source.TessellateCapCertified(cancelled, -1, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
