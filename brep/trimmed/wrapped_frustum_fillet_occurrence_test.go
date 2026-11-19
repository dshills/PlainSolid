//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestFilletedFrustumRationalOccurrence(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	r := big.NewRat
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
	q, e := PrepareFilletedFrustumMaterial(ctx, source, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	var saved bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &saved, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &saved, FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	// Rotate the cone axis into an oblique XZ direction, and translate by
	// genuinely nonbinary rationals. No support or boundary is rounded.
	m := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	point := [3]*big.Rat{r(837, 104), r(0, 1), r(15, 208)}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 0, 1} {
		p := point
		p[0] = new(big.Rat).Add(p[0], new(big.Rat).Mul(r(sign, 1), eps))
		world, e := pose.TransformPoint(ctx, p, false, 4096)
		if e != nil {
			t.Fatal(e)
		}
		got, e := placed.ClassifyRational(ctx, world)
		if e != nil {
			t.Fatal(e)
		}
		want := geom.BoreBoundary
		if sign < 0 {
			want = geom.BoreInside
		}
		if sign > 0 {
			want = geom.BoreOutside
		}
		if got.Location != want {
			t.Fatal(sign, got)
		}
		if sign == 0 && !reflect.DeepEqual(got.Faces, []int{q.fillets.Blends[0]}) {
			t.Fatal(got)
		}
	}
	d := [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}
	world, _ := pose.TransformPoint(ctx, point, false, 4096)
	wd, _ := pose.TransformVector(ctx, d, false, 4096)
	a, e := q.IntersectLineRational(ctx, point, d, r(-1, 1), r(8, 1), geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := placed.IntersectLineRational(ctx, world, wd, r(-1, 1), r(8, 1), geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Hits) != 2 || len(b.Hits) != len(a.Hits) || len(a.TrimmedAway) != len(b.TrimmedAway) {
		t.Fatal(a, b)
	}
	for i, h := range a.Hits {
		k := b.Hits[i]
		same, e := h.Parameter.Compare(ctx, k.Parameter, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !same.Proven || same.Order != 0 || !reflect.DeepEqual(h.Faces, k.Faces) || h.Kind != k.Kind || h.Before != k.Before || h.After != k.After {
			t.Fatal(i, same, e, h, k)
		}
	}
	native, e := q.Solid().MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := placed.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if mass.Volume != native.Volume || mass.Area != native.Area {
		t.Fatal("rigid invariants")
	}
	expected := geom.Vec3{X: .6*native.Centroid.X + .8*native.Centroid.Z + 1./3, Y: native.Centroid.Y - 2./7, Z: -.8*native.Centroid.X + .6*native.Centroid.Z + 5./11}
	if mass.Centroid.Sub(expected).Length() > 1e-13 {
		t.Fatal(mass.Centroid, expected)
	}
	// Independent rigid tensor invariants and explicit mixed XZ entry.
	trace := func(v [3][3]float64) float64 { return v[0][0] + v[1][1] + v[2][2] }
	expectedXZ := .48*(native.Inertia[2][2]-native.Inertia[0][0]) - .28*native.Inertia[0][2]
	if math.Abs(trace(mass.Inertia)-trace(native.Inertia)) > 1e-9 || math.Abs(mass.Inertia[0][2]-expectedXZ) > 1e-9 {
		t.Fatal(mass.Inertia, native.Inertia)
	}
	m[0][0].SetInt64(99)
	returned, _ := placed.Pose()
	returned[0][0].SetInt64(88)
	again, _ := placed.Pose()
	if again[0][0].Cmp(r(3, 5)) != 0 {
		t.Fatal("mutable pose")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := placed.ClassifyRational(cancelCtx, world); e != context.Canceled || v.Faces != nil {
		t.Fatal(v, e)
	}
	if v, e := placed.IntersectLineRational(ctx, world, wd, r(-1, 1), r(8, 1), geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); e == nil || len(v.Hits) != 0 {
		t.Fatal("atomic budget", v, e)
	}
	if _, e = q.PlaceRigid(ctx, placement.RationalRigid{}); e == nil {
		t.Fatal("zero pose")
	}
	// Proper rigid validation refuses reflection and arbitrary shear.
	bad, _ := placed.Pose()
	bad[1][1].Neg(bad[1][1])
	if _, e = placement.NewRationalRigid(bad, 4096); e == nil {
		t.Fatal("reflection")
	}
	bad, _ = placed.Pose()
	bad[0][1].SetInt64(1)
	if _, e = placement.NewRationalRigid(bad, 4096); e == nil {
		t.Fatal("shear")
	}
}
