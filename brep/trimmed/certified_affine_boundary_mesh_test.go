package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedAffineSplineHoledBoundary(t *testing.T) {
	ctx := context.Background()
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 3}, {Y: 3}}
	holes := [][]geom.Vec3{{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 1}}}
	planar, e := brep.PrismWithHoles(outer, holes, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	s := fromPlanar(t, planar, 0)
	faces := s.Faces()
	holed := map[int]bool{}
	for i, f := range faces {
		if len(f.Holes) > 0 {
			holed[i] = true
		}
		d, e := faceUVBounds(f)
		if e != nil {
			t.Fatal(e)
		}
		var poles []geom.Vec3
		for _, u := range []float64{d.UMin, d.UMax} {
			for _, v := range []float64{d.VMin, d.VMax} {
				p, e := f.Surface.Eval(u, v)
				if e != nil {
					t.Fatal(e)
				}
				poles = append(poles, p)
			}
		}
		bs, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{d.UMin, d.UMin, d.UMax, d.UMax}, []float64{d.VMin, d.VMin, d.VMax, d.VMax}, nil)
		if e != nil {
			t.Fatal(e)
		}
		faces[i].Surface = bs
	}
	edges := s.Edges()
	seenEdges := map[int]bool{}
	for _, f := range faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if seenEdges[u.Edge] {
					continue
				}
				seenEdges[u.Edge] = true
				a, b := u.UVStart, u.UVEnd
				if u.Reversed {
					a, b = b, a
				}
				edges[u.Edge].Curve = surfaceTrimCurve{f.Surface, a, b, nil}
				edges[u.Edge].Range = parametric.Interval{Max: 1}
			}
		}
	}
	s, e = NewSolid(s.Vertices(), edges, faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.TessellateAffineBoundaryCertified(ctx, AffineSplineShellBooleanOptions{}, 1000)
	if e != nil || !r.BothDirections {
		t.Fatal(e)
	}
	closed, err := s.TessellatePolygonShellCertified(ctx, CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: .01}})
	if err != nil || !closed.BothDirections || math.Abs(closed.Mesh.SignedVolume()-20) > 1e-12 {
		t.Fatal("conforming shell with inner wires", err)
	}
	adaptive, err := s.TessellateAdaptivePolygonShellCertified(ctx, CertifiedAdaptivePolygonShellOptions{Deflection: .01})
	if err != nil || !adaptive.BothDirections || adaptive.Rounds != 0 || math.Abs(adaptive.Mesh.SignedVolume()-20) > 1e-12 {
		t.Fatal("adaptive shell with inner wires", err)
	}
	if math.Abs(r.Mesh.SignedVolume()-20) > 1e-12 || math.Abs(r.Mesh.Area()-60) > 1e-12 {
		t.Fatal("analytic through-hole mass", r.Mesh.SignedVolume(), r.Mesh.Area())
	}

	for face := range holed {
		sheet, err := s.TessellateAffineQuadraticFaceCertified(ctx, face, AffineQuadraticTrimMeshOptions{Region: QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .01}}})
		if err != nil || math.Abs(sheet.Mesh.Area()-10) > 1e-12 {
			t.Fatal("existing holed face adapter", face, err)
		}
		for _, original := range sheet.OriginalFace {
			if original != face {
				t.Fatal("lost existing face ancestry")
			}
		}
	}
	if len(holed) != 2 {
		t.Fatal("fixture missing actual inner wires")
	}
	seen := map[int]bool{}
	for i, tri := range r.Mesh.Triangles {
		face := r.OriginalFace[i]
		if face < 0 || face >= len(faces) || r.SquaredErrors[i].Sign() != 0 {
			t.Fatal("false ancestry/certificate")
		}
		seen[face] = true
		if holed[face] {
			a, b, c := r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]
			center := a.Add(b).Add(c).Scale(1.0 / 3)
			if center.X > 1 && center.X < 3 && center.Y > 1 && center.Y < 2 {
				t.Fatal("triangle bridged cap hole", tri)
			}
		}
	}
	if len(seen) != len(faces) {
		t.Fatal("lost original face")
	}
	denied, e := s.TessellateAffineBoundaryCertified(ctx, AffineSplineShellBooleanOptions{}, 1)
	if e == nil || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("partial triangle budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = s.TessellateAffineBoundaryCertified(canceled, AffineSplineShellBooleanOptions{}, 0)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
