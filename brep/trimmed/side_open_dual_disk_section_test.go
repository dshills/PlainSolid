package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestExactSameSideDualDiskSection(t *testing.T) {
	ctx := context.Background()
	min, max := geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}}
	r, e := CertifySideOpenDualDiskSection(ctx, min, max, discs, trimvalidate.Options{})
	if e != nil || len(r.Outer) != 9 || r.Join[0].Cmp(big.NewRat(5, 1)) != 0 || r.Join[1].Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal(r, e)
	}
	var loop trimvalidate.Loop
	for i, q := range r.Outer {
		a, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, a)
		if id := r.Owner[i]; id >= 0 {
			for j := int64(1); j < 11; j++ {
				p, _, e := q.EvaluateRational(big.NewRat(j, 11))
				if e != nil {
					t.Fatal(e)
				}
				d2 := func(c ExtrusionCircle) *big.Rat {
					x := new(big.Rat).Sub(p[0], ratInset(c.Center.X))
					y := new(big.Rat).Sub(p[1], ratInset(c.Center.Y))
					return new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
				}
				r2 := big.NewRat(25, 4)
				if d2(discs[id]).Cmp(r2) != 0 || d2(discs[1-id]).Cmp(r2) <= 0 || p[1].Cmp(big.NewRat(4, 1)) <= 0 {
					t.Fatal("wrong exact exposed circular arc", i, p)
				}
			}
		}
	}
	c, proof, e := trimvalidate.CertifyLoops(ctx, loop, nil, trimvalidate.Options{})
	if e != nil || proof.Status != trimvalidate.Certified {
		t.Fatal(proof, e)
	}
	a, e := c.BoundSignedArea(ctx, big.NewRat(1, 1000))
	if e != nil {
		t.Fatal(e)
	}
	l, _ := a.Lower.Float64()
	u, _ := a.Upper.Float64()
	// Independent two overlapping disks, cut in half by their centerline.
	want := 60 - (25./4*(math.Pi-math.Acos(3./5)) + 3)
	if want < l || u < want {
		t.Fatal("remaining section area", l, u, want)
	}
	bad := discs
	bad[0].Center.X = 6
	if r, e := CertifySideOpenDualDiskSection(ctx, min, max, bad, trimvalidate.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(r.Outer) != 0 {
		t.Fatal("irrational circle crossing rounded", r, e)
	}
}

func TestExactSameSideDualDiskPrism(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	for _, depth := range []float64{3, 6} {
		r, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil || len(r.WallFaces[0]) != 1 || len(r.WallFaces[1]) != 1 || (r.FloorFace >= 0) != (depth == 3) {
			t.Fatal("two wall owners and common floor", depth, e)
		}
		x := math.Pi - math.Acos(.6)
		b := 6.25*x + 3
		wantArea := 2*60 + 2*(10+6)*6 - 8*depth + 5*x*depth
		if depth == 6 {
			wantArea -= 2 * b
		}
		wantVolume := 360 - depth*b
		props, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(props.Volume-wantVolume) > 3e-7 || math.Abs(props.Area-wantArea) > 3e-7 {
			t.Fatal("independent two-notch measure", depth, props, wantVolume, wantArea, e)
		}
		wantCentroid := geom.Vec3{X: 5, Y: (360*7 - depth*(4*b+56./3)) / wantVolume, Z: (1080 - depth*depth*b/2) / wantVolume}
		if props.Centroid.Sub(wantCentroid).Length() > 3e-8 {
			t.Fatal("independent two-notch centroid", props.Centroid, wantCentroid)
		}
		var stream bytes.Buffer
		if e := WriteJSON(&stream, r.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &stream, JSONLimits{})
		if e != nil {
			t.Fatal("native revalidation", e)
		}
		if len(reload.Topology().Shells) != 1 {
			t.Fatal("reloaded exact disk prism topology")
		}
		for _, row := range r.WallFaces {
			i := row[0]
			if !geom.SameSplineSurface(r.Solid.faces[i].Surface.(*geom.BSplineSurface), reload.faces[i].Surface.(*geom.BSplineSurface)) {
				t.Fatal("native reload changed the unnormalized exact circle weights")
			}
		}
	}
}
