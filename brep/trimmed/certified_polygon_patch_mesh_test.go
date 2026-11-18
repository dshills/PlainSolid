package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedCurvedPolygonHoleMesh(t *testing.T) {
	ctx := context.Background()
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	surface, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	outer := []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	holes := [][]geom.Vec2{{{X: .25, Y: .25}, {X: .25, Y: .75}, {X: .75, Y: .75}, {X: .75, Y: .25}}}
	o := CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 16}
	r, e := TessellatePolygonPatchCertified(ctx, patch, outer, holes, o)
	if e != nil || !r.BothDirections || len(r.Loops) != 2 || r.OriginalFace != -1 {
		t.Fatal(r.Subdivisions, e)
	}
	for i, uv := range r.VertexUV {
		p := r.Mesh.Vertices[i]
		if p.X != uv.X || p.Y != uv.Y || p.Z != uv.X*uv.X+uv.Y*uv.Y {
			t.Fatal("independent parabolic surface sample", uv, p)
		}
	}
	edges := map[[2]int]int{}
	area := 0.
	key := func(a, b int) [2]int {
		if a > b {
			a, b = b, a
		}
		return [2]int{a, b}
	}
	for i, tri := range r.Mesh.Triangles {
		for j, a := range tri {
			edges[key(a, tri[(j+1)%3])]++
		}
		uv := r.UVTriangles[i]
		area += ((uv[1].X-uv[0].X)*(uv[2].Y-uv[0].Y) - (uv[1].Y-uv[0].Y)*(uv[2].X-uv[0].X)) / 2
		c := geom.Vec2{X: (uv[0].X + uv[1].X + uv[2].X) / 3, Y: (uv[0].Y + uv[1].Y + uv[2].Y) / 3}
		if c.X > .25 && c.X < .75 && c.Y > .25 && c.Y < .75 {
			t.Fatal("hole bridge")
		}
	}
	if area != .75 {
		t.Fatal("exact trimmed UV area", area)
	}
	boundary := map[[2]int]bool{}
	for _, loop := range r.Loops {
		for i, a := range loop {
			boundary[key(a, loop[(i+1)%len(loop)])] = true
		}
	}
	for edge, n := range edges {
		if boundary[edge] && n != 1 || !boundary[edge] && n != 2 {
			t.Fatal("nonconforming boundary/interior edge", edge, n)
		}
	}
	if len(r.Mesh.Vertices)-len(edges)+len(r.Mesh.Triangles) != 0 {
		t.Fatal("hole topology lost")
	}
	bad, e := TessellatePolygonPatchCertified(ctx, patch, outer, holes, CertifiedPatchMeshOptions{Deflection: .0001, MaxTriangleChecks: 1})
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial budget", e)
	}
	invalid := [][]geom.Vec2{{{X: .5, Y: .5}, {X: .5, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: .5}}}
	_, e = TessellatePolygonPatchCertified(ctx, patch, outer, invalid, o)
	if e == nil {
		t.Fatal("touching hole accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = TessellatePolygonPatchCertified(canceled, patch, outer, holes, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestCertifiedRationalPolygonHoleMesh(t *testing.T) {
	slowtest.Skip(t)
	poles := []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}
	weights := []float64{1, 2, 3, 1}
	knots := []float64{0, 0, 1, 1}
	surface, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, knots, knots, weights)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	outer := []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	holes := [][]geom.Vec2{{{X: .25, Y: .25}, {X: .25, Y: .75}, {X: .75, Y: .75}, {X: .75, Y: .25}}}
	result, err := TessellatePolygonPatchCertified(context.Background(), patch, outer, holes, CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 32})
	if err != nil || !result.BothDirections {
		t.Fatal(err)
	}
	evaluate := func(uv geom.Vec2) geom.Vec3 {
		b := []float64{(1 - uv.X) * (1 - uv.Y), (1 - uv.X) * uv.Y, uv.X * (1 - uv.Y), uv.X * uv.Y}
		p := geom.Vec3{}
		w := 0.
		for i := range b {
			w += b[i] * weights[i]
			p = p.Add(poles[i].Scale(b[i] * weights[i]))
		}
		return p.Scale(1 / w)
	}
	for i, tri := range result.Mesh.Triangles {
		uv := result.UVTriangles[i]
		q := geom.Vec2{X: (uv[0].X + uv[1].X + uv[2].X) / 3, Y: (uv[0].Y + uv[1].Y + uv[2].Y) / 3}
		linear := result.Mesh.Vertices[tri[0]].Add(result.Mesh.Vertices[tri[1]]).Add(result.Mesh.Vertices[tri[2]]).Scale(1.0 / 3)
		delta := evaluate(q).Sub(linear)
		bound, _ := result.SquaredErrors[i].Float64()
		if delta.Dot(delta) > bound+1e-14 {
			t.Fatal("independent rational correspondence", i, delta.Dot(delta), bound)
		}
	}
	for _, id := range result.Loops[1] {
		uv := result.VertexUV[id]
		if uv.X != .25 && uv.X != .75 && uv.Y != .25 && uv.Y != .75 {
			t.Fatal("hole sample moved", uv)
		}
		if evaluate(uv).Sub(result.Mesh.Vertices[id]).Length() > 1e-14 {
			t.Fatal("rational boundary sample")
		}
	}
}
