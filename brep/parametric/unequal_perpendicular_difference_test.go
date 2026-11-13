package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestUnequalPerpendicularDifference(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	out, e := UnequalPerpendicularCylinderDifference(ctx, center, 5, 3, 4, 6, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	const n = 8192
	step := math.Pi / n
	var common, stockBuried, cutterWall float64
	for i := 0; i <= n; i++ {
		u := -math.Pi/2 + float64(i)*step
		sn, cs := math.Sincos(u)
		root := math.Sqrt(25 - 9*sn*sn)
		weight := 2.
		if i == 0 || i == n {
			weight = 1
		} else if i%2 == 1 {
			weight = 4
		}
		common += weight * 36 * cs * cs * root
		stockBuried += weight * 180 * cs * cs / root
		cutterWall += weight * 12 * root
	}
	common *= step / 3
	stockBuried *= step / 3
	cutterWall *= step / 3
	wantV := 200*math.Pi - common
	wantA := 130*math.Pi - stockBuried + cutterWall
	if math.Abs(p.Volume-wantV) > 1e-6 || math.Abs(p.Area-wantA) > 1e-6 || p.Centroid.Sub(center).Length() > 1e-8 {
		t.Fatal(p, wantV, wantA)
	}
	if len(out.StockWallFaces) != 12 || len(out.CutterWallFaces) != 2 || len(out.LowerCapFaces) != 6 || len(out.UpperCapFaces) != 6 {
		t.Fatal("ancestry", out)
	}

	seen := make(map[int]bool)
	for group, ids := range [][]int{out.StockWallFaces, out.CutterWallFaces, out.LowerCapFaces, out.UpperCapFaces} {
		for _, id := range ids {
			if seen[id] {
				t.Fatal("duplicate ancestry", id)
			}
			seen[id] = true
			f := out.Solid.Faces()[id]
			for _, a := range []float64{0, .25, .5, .75, 1} {
				for _, b := range []float64{0, .5, 1} {
					v, err := f.Surface.Eval(f.U.Min+a*(f.U.Max-f.U.Min), f.V.Min+b*(f.V.Max-f.V.Min))
					if err != nil {
						t.Fatal(err)
					}
					q := v.Sub(center)
					var residual float64
					switch group {
					case 0:
						residual = q.Y*q.Y + q.Z*q.Z - 25
					case 1:
						residual = q.X*q.X + q.Y*q.Y - 9
					case 2:
						residual = q.X + 4
					case 3:
						residual = q.X - 4
					}
					if math.Abs(residual) > 1e-12 {
						t.Fatal("source support", group, id, residual)
					}
				}
			}
		}
	}
	if len(seen) != len(out.Solid.Faces()) {
		t.Fatal("missing ancestry")
	}
	for _, id := range out.CutterWallFaces {
		f := out.Solid.Faces()[id]
		v, e := f.Surface.Eval(0, 0)
		if e != nil {
			t.Fatal(e)
		}
		a, b, e := f.Surface.Partials(0, 0)
		if e != nil {
			t.Fatal(e)
		}
		normal := a.Cross(b)
		if f.Reversed {
			normal = normal.Scale(-1)
		}
		radial := v.Sub(center)
		radial.Z = 0
		if normal.Dot(radial) >= 0 {
			t.Fatal("cavity points outward")
		}
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, out.Solid); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := restored.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || mass != p {
		t.Fatal("reload", e)
	}
	mesh, e := restored.Tessellate(ctx, TessellationOptions{Deflection: 1, AnalyticBound: true})
	if e != nil || mesh.ValidateSolid() != nil {
		t.Fatal("mesh", e)
	}
	edges := map[[2]int]bool{}
	for _, tri := range mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	if len(mesh.Vertices)-len(edges)+len(mesh.Triangles) != 0 {
		t.Fatal("expected genus one")
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("contacts", e, len(contacts.Contacts))
	}
	t.Log(p.Volume, p.Area, len(mesh.Triangles))
}

func TestUnequalPerpendicularDifferenceLimits(t *testing.T) {
	for _, args := range [][4]float64{{5, 2, 4, 6}, {5, 3, 3, 6}, {5, 3, 4, 5}, {3, 5, 6, 4}, {5, 3, 4, math.Inf(1)}} {
		out, err := UnequalPerpendicularCylinderDifference(context.Background(), geom.Vec3{}, args[0], args[1], args[2], args[3], 1e-8)
		if err == nil || len(out.Solid.Faces()) != 0 || out.StockVolumePiCoefficient != nil {
			t.Fatal("partial or accepted", args, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := UnequalPerpendicularCylinderDifference(ctx, geom.Vec3{}, 5, 3, 4, 6, 1e-8)
	if !errors.Is(err, context.Canceled) || len(out.Solid.Faces()) != 0 {
		t.Fatal(err)
	}
}
