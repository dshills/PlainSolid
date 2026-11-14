package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestWrappedQuadraticFaceJSON(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	face, e := NewWrappedQuadraticFace(ctx, w, wrappedQuadraticLoop(t), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: true})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedQuadraticFaceJSON(ctx, &out, face, WrappedQuadraticFaceJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	decoded, e := ReadWrappedQuadraticFaceJSON(ctx, bytes.NewReader(raw), WrappedQuadraticFaceJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	f := decoded.NativeFace()
	if !f.Face.Reversed || len(f.Edges) != 4 {
		t.Fatal("lost source face")
	}
	open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
	proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal("reload edge proof", e)
	}
	p, e := decoded.Composition(0, 2)
	if e != nil || len(p.Pieces) != 2 || p.Pieces[0].End.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("lost exact source seam", e)
	}
	line, e := decoded.IntersectInclinedLine(ctx, geom.Vec3{}, geom.Vec3{X: 3, Z: .5}, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfacePatchOptions{})
	if e != nil || len(line.Hits) != 1 || line.Hits[0].UV[1].Cmp(big.NewRat(1, 3)) != 0 || line.Hits[0].CrossingSign != -1 {
		t.Fatal("reload original model query", e)
	}
	out.Reset()
	if e := WriteWrappedQuadraticFaceJSON(ctx, &out, decoded, WrappedQuadraticFaceJSONOptions{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("nondeterministic lossless recipe", e)
	}
	low := WrappedQuadraticFaceJSONOptions{}
	low.Face.MaxOperations = decoded.ExactOperations() - 1
	if q, e := ReadWrappedQuadraticFaceJSON(ctx, bytes.NewReader(raw), low); e == nil || q != nil {
		t.Fatal("partial low-work decode")
	}
	for _, limits := range []WrappedQuadraticFaceJSONOptions{{MaxBytes: 1}, {MaxControlPoints: 4}, {Face: WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{MaxOperations: 1}}}, {Face: WrappedQuadraticFaceOptions{MaxEdges: 3}}} {
		out.Reset()
		if e := WriteWrappedQuadraticFaceJSON(ctx, &out, face, limits); e == nil || out.Len() != 0 {
			t.Fatal("writer published before proof", e)
		}
		if q, e := ReadWrappedQuadraticFaceJSON(ctx, bytes.NewReader(raw), limits); e == nil || q != nil {
			t.Fatal("reader resource bypass")
		}
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	out.Reset()
	if e := WriteWrappedQuadraticFaceJSON(cancel, &out, face, WrappedQuadraticFaceJSONOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("cancel writer")
	}
	if q, e := ReadWrappedQuadraticFaceJSON(cancel, bytes.NewReader(raw), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
		t.Fatal("cancel reader")
	}
}
func TestWrappedQuadraticFaceJSONRejectsClaimsAndDamage(t *testing.T) {
	ctx := context.Background()
	face, e := NewWrappedQuadraticFace(ctx, wrappedOverlapSupport(t), wrappedQuadraticLoop(t), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedQuadraticFaceJSON(ctx, &out, face, WrappedQuadraticFaceJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	raw := out.Bytes()
	for _, bad := range []string{strings.Replace(string(raw), "{", "{\"Complete\":true,", 1), strings.Replace(string(raw), "{", "{\"version\":1,", 1), strings.Replace(string(raw), "{", "{\"VERSION\":1,", 1), string(raw) + "{}"} {
		if q, e := ReadWrappedQuadraticFaceJSON(ctx, strings.NewReader(bad), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
			t.Fatal("ambiguous/asserted recipe accepted")
		}
	}
	mutations := []func(*wrappedQuadraticFaceRecipe){func(n *wrappedQuadraticFaceRecipe) { n.Surface.Poles[len(n.Surface.Poles)-1].X += .125 }, func(n *wrappedQuadraticFaceRecipe) { n.Surface.CountU = 1 << 30 }, func(n *wrappedQuadraticFaceRecipe) { n.Surface.DegreeU = 17 }, func(n *wrappedQuadraticFaceRecipe) { n.Loops[0][0][0][0] = "1e1000000000" }, func(n *wrappedQuadraticFaceRecipe) { n.Loops[0][0][0][0] = "1/0" }, func(n *wrappedQuadraticFaceRecipe) { n.Loops[0][0][0][0] = "00" }, func(n *wrappedQuadraticFaceRecipe) { n.Loops[0][0] = append(n.Loops[0][0], []string{"0", "0", "1"}) }, func(n *wrappedQuadraticFaceRecipe) { n.ProjectionAxes = append(n.ProjectionAxes, 2) }, func(n *wrappedQuadraticFaceRecipe) { n.Loops[0][0][0][2] = "0" }, func(n *wrappedQuadraticFaceRecipe) { n.Loops[0][0][0][0] = "1" }, func(n *wrappedQuadraticFaceRecipe) {
		n.Surface.Weights = make([]float64, len(n.Surface.Poles))
		for i := range n.Surface.Weights {
			n.Surface.Weights[i] = 2
		}
	}, func(n *wrappedQuadraticFaceRecipe) { n.ProjectionAxes = []int{0, 1} }}
	for i, mutate := range mutations {
		var n wrappedQuadraticFaceRecipe
		if e := json.Unmarshal(raw, &n); e != nil {
			t.Fatal(e)
		}
		mutate(&n)
		bad, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		if q, e := ReadWrappedQuadraticFaceJSON(ctx, bytes.NewReader(bad), WrappedQuadraticFaceJSONOptions{}); e == nil || q != nil {
			t.Fatal("damaged source recipe accepted", i, e)
		}
	}
}

func TestWrappedQuadraticFaceJSONHoleLift(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	outer := wrappedQuadraticLoop(t)
	pts := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, p := range pts {
		q, e := quadraticLine(p, pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	for _, loop := range [][]ExactQuadraticUV{outer, hole} {
		for i, q := range loop {
			h := q.HomogeneousControls()
			for j := range h {
				h[j][0].Add(h[j][0], new(big.Rat).Mul(big.NewRat(4, 1), h[j][2]))
			}
			var e error
			loop[i], e = NewExactQuadraticUV(h)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	face, e := NewWrappedQuadraticFace(ctx, w, outer, [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	var buffer bytes.Buffer
	if e := WriteWrappedQuadraticFaceJSON(ctx, &buffer, face, WrappedQuadraticFaceJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	round, e := ReadWrappedQuadraticFaceJSON(ctx, &buffer, WrappedQuadraticFaceJSONOptions{})
	if e != nil {
		t.Fatal(e)
	}
	f := round.NativeFace()
	if len(f.Face.Holes) != 1 || f.Face.Outer[0].UVStart.X != 3.75 {
		t.Fatal("original lifted loops not preserved")
	}
	hit, e := round.IntersectInclinedLine(ctx, geom.Vec3{X: -3, Z: -1.25}, geom.Vec3{X: 2, Z: .25}, [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}, geom.SurfacePatchOptions{})
	if e != nil || !hit.Complete || len(hit.Hits) != 0 || hit.ExcludedByTrim != 2 {
		t.Fatal("reloaded original hole", hit, e)
	}
}
