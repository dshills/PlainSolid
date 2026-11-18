package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestEmbeddedNativeHoledPrism(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	var faces []ExactQuadraticShellFace
	var surfaces []*geom.BSplineSurface
	patch := func(poles []geom.Vec3, du, dv, nu, nv int) *geom.SurfacePatch {
		ku := make([]float64, 2*(du+1))
		kv := make([]float64, 2*(dv+1))
		for i := du + 1; i < len(ku); i++ {
			ku[i] = 1
		}
		for i := dv + 1; i < len(kv); i++ {
			kv[i] = 1
		}
		s, e := geom.NewBSplineSurface(du, dv, nu, nv, poles, ku, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, s)
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	trim := func(a, b, c geom.Vec2) ExactQuadraticUV {
		h := [3][3]*big.Rat{}
		for i, p := range []geom.Vec2{a, b, c} {
			h[i] = [3]*big.Rat{ratInset(p.X), ratInset(p.Y), big.NewRat(1, 1)}
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	loops := func(r float64, reverse, reflect bool) []ExactQuadraticUV {
		v := []geom.Vec2{{X: .5 + r, Y: .5}, {X: .5, Y: .5 + r}, {X: .5 - r, Y: .5}, {X: .5, Y: .5 - r}}
		var out []ExactQuadraticUV
		for j := 0; j < 4; j++ {
			k := j
			if reverse {
				k = 3 - j
			}
			a, b := v[k], v[(k+1)%4]
			c := geom.Vec2{X: a.X + b.X - .5, Y: a.Y + b.Y - .5}
			if reverse {
				a, b = b, a
			}
			if reflect {
				a.X = 1 - a.X
				b.X = 1 - b.X
				c.X = 1 - c.X
			}
			out = append(out, trim(a, c, b))
		}
		return out
	}
	for _, bottom := range []bool{false, true} {
		z := 1.
		if bottom {
			z = 0
		}
		poles := []geom.Vec3{}
		for _, u := range []float64{0, 1} {
			for _, v := range []float64{0, 1} {
				x := u
				if bottom {
					x = 1 - u
				}
				poles = append(poles, geom.Vec3{X: x, Y: v, Z: z})
			}
		}
		faces = append(faces, ExactQuadraticShellFace{Patch: patch(poles, 1, 1, 2, 2), Outer: loops(.375, bottom, bottom), Holes: [][]ExactQuadraticUV{loops(.125, !bottom, bottom)}})
	}
	for _, inner := range []bool{false, true} {
		r := .375
		if inner {
			r = .125
		}
		for _, arc := range loops(r, inner, false) {
			h := arc.HomogeneousControls()
			var poles []geom.Vec3
			for i := 0; i < 3; i++ {
				x, _ := h[i][0].Float64()
				y, _ := h[i][1].Float64()
				for _, z := range []float64{0, 1} {
					poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
				}
			}
			uv := []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
			var outer []ExactQuadraticUV
			for j, a := range uv {
				b := uv[(j+1)%4]
				outer = append(outer, trim(a, geom.Vec2{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}, b))
			}
			faces = append(faces, ExactQuadraticShellFace{Patch: patch(poles, 2, 1, 3, 2), Outer: outer})
		}
	}
	type use struct {
		face, trim int
		a, b       string
	}
	pending := map[[2]string]use{}
	var seams []QuadraticTrimStitchSeam
	var endpointKeys [][2]string
	for fi, f := range faces {
		flat := append(append([]ExactQuadraticUV{}, f.Outer...), func() []ExactQuadraticUV {
			var x []ExactQuadraticUV
			for _, h := range f.Holes {
				x = append(x, h...)
			}
			return x
		}()...)
		for ti, c := range flat {
			var keys [2]string
			for k := 0; k < 2; k++ {
				uv, _, e := c.EvaluateRational(big.NewRat(int64(k), 1))
				if e != nil {
					t.Fatal(e)
				}
				p, e := f.Patch.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				keys[k] = p.Point[0].RatString() + "," + p.Point[1].RatString() + "," + p.Point[2].RatString()
			}
			key := keys
			if key[0] > key[1] {
				key[0], key[1] = key[1], key[0]
			}
			if old, ok := pending[key]; ok {
				if old.a != keys[1] || old.b != keys[0] {
					t.Fatal("orientation")
				}
				seams = append(seams, QuadraticTrimStitchSeam{FaceA: old.face, TrimA: old.trim, FaceB: fi, TrimB: ti})
				endpointKeys = append(endpointKeys, [2]string{old.a, old.b})
				delete(pending, key)
			} else {
				pending[key] = use{fi, ti, keys[0], keys[1]}
			}
		}
	}
	if len(pending) != 0 {
		t.Fatal("open fixture")
	}
	var junctions [][]QuadraticTrimSeamEndpoint
	ids := map[string]int{}
	for si, ends := range endpointKeys {
		for k, key := range ends {
			id, ok := ids[key]
			if !ok {
				id = len(junctions)
				ids[key] = id
				junctions = append(junctions, nil)
			}
			junctions[id] = append(junctions[id], QuadraticTrimSeamEndpoint{Seam: si, End: k})
		}
	}
	o := ExactQuadraticShellMeshOptions{Join: ExactQuadraticFaceJoinOptions{Face: ExactQuadraticRegionMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}}
	got, e := StitchEmbeddedExactQuadraticShellCertified(ctx, faces, seams, junctions, o)
	if e != nil {
		t.Fatal(e)
	}
	volume, _ := got.SignedDisplayVolume.Float64()
	want := 5. / 12
	if math.Abs(volume-want) > .02 {
		t.Fatal("analytic volume", volume, want)
	}
	edges := map[[2]int]bool{}
	for _, tri := range got.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(got.Mesh.Vertices)-len(edges)+len(got.Mesh.Triangles) != 0 {
		t.Fatal("genus one")
	}
	// The same source is stored through the public topology constructor.
	vertices := make([]geom.Vec3, len(ids))
	storedFaces := make([]Face, len(faces))
	storedEdges := make([]parametric.Edge, len(seams))
	type side struct {
		edge    int
		reverse bool
	}
	ownership := map[[2]int]side{}
	for ei, seam := range seams {
		ownership[[2]int{seam.FaceA, seam.TrimA}] = side{ei, false}
		ownership[[2]int{seam.FaceB, seam.TrimB}] = side{ei, true}
	}
	for fi, f := range faces {
		storedFaces[fi].Surface = surfaces[fi]
		ti := 0
		for li, loop := range append([][]ExactQuadraticUV{f.Outer}, f.Holes...) {
			var uses []Use
			for _, curve := range loop {
				own := ownership[[2]int{fi, ti}]
				a, e := curve.EvalUV(0)
				if e != nil {
					t.Fatal(e)
				}
				b, e := curve.EvalUV(1)
				if e != nil {
					t.Fatal(e)
				}
				uses = append(uses, Use{Edge: own.edge, Reversed: own.reverse, UVStart: a, UVEnd: b, Curve: curve})
				if !own.reverse {
					ends := endpointKeys[own.edge]
					start, end := ids[ends[0]], ids[ends[1]]
					pa, _ := surfaces[fi].Eval(a.X, a.Y)
					pb, _ := surfaces[fi].Eval(b.X, b.Y)
					vertices[start] = pa
					vertices[end] = pb
					storedEdges[own.edge] = parametric.Edge{Start: start, End: end, Curve: surfaceTrimCurve{surfaces[fi], a, b, curve}, Range: parametric.Interval{Max: 1}}
				}
				ti++
			}
			if li == 0 {
				storedFaces[fi].Outer = uses
			} else {
				storedFaces[fi].Holes = append(storedFaces[fi].Holes, uses)
			}
		}
	}
	solid, e := NewSolid(vertices, storedEdges, storedFaces, 1e-9)
	if e != nil {
		t.Fatal("stored source", e)
	}
	adapted, e := solid.TessellateExactQuadraticShellCertified(ctx, SolidExactQuadraticMeshOptions{Shell: o})
	if e != nil {
		t.Fatal("adapter", e)
	}
	if adapted.ModelEdgeUsesCertified != 2*len(seams) || adapted.SignedDisplayVolume.Cmp(got.SignedDisplayVolume) != 0 {
		t.Fatal("adapter ancestry or geometry changed")
	}

	limited := o
	limited.MaxTriangles = 100
	partial, err := StitchEmbeddedExactQuadraticShellCertified(ctx, faces, seams, junctions, limited)
	if err == nil || len(partial.Mesh.Triangles) != 0 {
		t.Fatal("aggregate budget returned partial shell")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = solid.TessellateExactQuadraticShellCertified(cancelled, SolidExactQuadraticMeshOptions{Shell: o})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}
	for fi := 0; fi < 2; fi++ {
		for j := 4; j < 8; j++ {
			if adapted.TrimLoops[fi][j] != 1 || !adapted.TrimReversed[fi][j] || adapted.SourceParameters[fi][j].Scale.Cmp(big.NewRat(-1, 1)) != 0 {
				t.Fatal("hole traversal ancestry")
			}
		}
	}

	t.Log("triangles", len(got.Mesh.Triangles), "volume", volume, "exact source", want)
}
