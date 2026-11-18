package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func TestFilletedFrustumMaterialNativeEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}
	q, e := PrepareFilletedFrustumMaterial(ctx, source, 15./16, 15./8, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	cuts[0].Radius = 7
	var saved bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &saved, q); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadFilletedFrustumMaterialJSON(ctx, bytes.NewReader(saved.Bytes()), FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again, before, after bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &again, loaded); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(saved.Bytes(), again.Bytes()) {
		t.Fatal("canonical recipe")
	}
	if e = WriteJSON(&before, q.Solid()); e != nil {
		t.Fatal(e)
	}
	if e = WriteJSON(&after, loaded.Solid()); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("native entity IDs/geometry changed")
	}
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	p := [3]*big.Rat{r(837, 104), r(0, 1), r(15, 208)}
	hit, e := loaded.ClassifyRational(ctx, p)
	if e != nil || hit.Location != geom.BoreBoundary || len(hit.Faces) != 1 || hit.Faces[0] != q.fillets.Blends[0] {
		t.Fatal("independent torus boundary", hit, e)
	}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 1} {
		probe := p
		probe[0] = new(big.Rat).Add(p[0], new(big.Rat).Mul(r(sign, 1), eps))
		v, e := loaded.ClassifyRational(ctx, probe)
		want := geom.BoreInside
		if sign > 0 {
			want = geom.BoreOutside
		}
		if e != nil || v.Location != want {
			t.Fatal("sub-ULP original material", v, e)
		}
	}
	floor, e := loaded.ClassifyRational(ctx, [3]*big.Rat{r(2, 1), r(0, 1), r(1, 1)})
	if e != nil || floor.Location != geom.BoreBoundary || floor.Faces[0] != q.cuts[1].floor {
		t.Fatal("blind floor native ID", floor, e)
	}
	line, e := loaded.IntersectLineRational(ctx, p, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}, r(-1, 1), r(8, 1), geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(line.Hits) != 2 || line.Hits[0].Kind != "entry" || line.Hits[1].Kind != "exit" || line.Hits[0].Faces[0] != q.fillets.Blends[0] {
		t.Fatal("oblique after load", line, e)
	}
	root, e := line.Hits[0].Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || root.Rational == nil || root.Rational.Sign() != 0 {
		t.Fatal("known algebraic parameter", root, e)
	}
	// Valid geometry changes must not silently substitute another material.
	for _, change := range []func(*nativeFilletedFrustumMaterial){func(n *nativeFilletedFrustumMaterial) { n.Cuts[0].Radius = .25 }, func(n *nativeFilletedFrustumMaterial) { n.Cuts[1].Depth = .5 }, func(n *nativeFilletedFrustumMaterial) { n.Cuts[0].Radius = 10 }, func(n *nativeFilletedFrustumMaterial) { n.Binding = strings.Repeat("0", 64) }, func(n *nativeFilletedFrustumMaterial) {
		f := source.Faces()
		reordered, e := NewSolid(source.Vertices(), source.Edges(), []Face{f[2], f[0], f[1]}, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, reordered); e != nil {
			t.Fatal(e)
		}
		n.Stock = b.Bytes()
	}} {
		var n nativeFilletedFrustumMaterial
		if e = json.Unmarshal(saved.Bytes(), &n); e != nil {
			t.Fatal(e)
		}
		change(&n)
		raw, e := json.Marshal(n)
		if e != nil {
			t.Fatal(e)
		}
		bad, e := ReadFilletedFrustumMaterialJSON(ctx, bytes.NewReader(raw), FilletedFrustumMaterialJSONLimits{})
		if e == nil || bad.Solid().Topology().Faces != 0 || bad.recipe != nil {
			t.Fatal("tampered recipe accepted", e)
		}
	}
	for _, raw := range [][]byte{append(append([]byte(nil), saved.Bytes()...), []byte("{}")...), bytes.Replace(saved.Bytes(), []byte(`"version":1`), []byte(`"version":1,"trusted":true`), 1)} {
		bad, e := ReadFilletedFrustumMaterialJSON(ctx, bytes.NewReader(raw), FilletedFrustumMaterialJSONLimits{})
		if e == nil || bad.recipe != nil {
			t.Fatal("strict envelope", e)
		}
	}
	for _, limits := range []FilletedFrustumMaterialJSONLimits{{MaxBytes: 32}, {Stock: JSONLimits{MaxEntities: 1}}, {Proof: geom.SurfacePatchOptions{MaxOperations: 1}}} {
		bad, e := ReadFilletedFrustumMaterialJSON(ctx, bytes.NewReader(saved.Bytes()), limits)
		if e == nil || bad.recipe != nil {
			t.Fatal("caller limits", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := ReadFilletedFrustumMaterialJSON(cancelled, bytes.NewReader(saved.Bytes()), FilletedFrustumMaterialJSONLimits{})
	if e != context.Canceled || bad.recipe != nil {
		t.Fatal("cancel", e)
	}
}
