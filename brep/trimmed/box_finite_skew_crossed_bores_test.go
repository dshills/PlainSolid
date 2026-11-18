package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestFiniteSkewCrossedBores(t *testing.T) {
	slowtest.Skip(t)
	dims, c := geom.Vec3{X: 30, Y: 24, Z: 28}, geom.Vec3{X: 15, Y: 12, Z: 14}
	for _, r := range [][2]float64{{4, 2}, {2, 4}} {
		for _, offset := range []float64{0, .5} {
			for _, normal := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
				for _, xo := range []bool{false, true} {
					for _, zo := range []bool{false, true} {
						for _, depths := range [][2]float64{{21, 0}, {0, 20}, {21, 20}} {
							f, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: -4, Z: 7}, normal)
							through, e := BoxSkewCrossedBores(context.Background(), f, dims, c, r[0], r[1], offset, 1e-9)
							if e != nil {
								t.Fatal(e)
							}
							out, e := BoxFiniteSkewCrossedBores(context.Background(), f, dims, c, r[0], r[1], offset, depths[0], depths[1], xo, zo, 1e-9)
							if e != nil {
								t.Fatal(r, offset, normal, xo, zo, depths, e)
							}
							n := 16
							v := dims.X*dims.Y*dims.Z - through.RemovedVolume
							a := 2*(dims.X*dims.Y+dims.X*dims.Z+dims.Y*dims.Z) + through.SurfaceAreaDelta
							for i, d := range depths {
								if d == 0 {
									continue
								}
								length := dims.X
								if i == 1 {
									length = dims.Z
								}
								v += math.Pi * r[i] * r[i] * (length - d)
								a += -2*math.Pi*r[i]*(length-d) + 2*math.Pi*r[i]*r[i]
								n++
							}
							if len(out.Solid.Faces()) != n || depths[0] == 0 && out.XFloorFace != -1 || depths[0] != 0 && out.XFloorFace != 16 || depths[1] == 0 && out.ZFloorFace != -1 || depths[1] != 0 && out.ZFloorFace != n-1 {
								t.Fatal("indices", out.XFloorFace, out.ZFloorFace, n)
							}
							mass, e := out.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
							if e != nil || math.Abs(mass.Volume-v) > 1e-6 || math.Abs(mass.Area-a) > 1e-6 {
								t.Fatal(mass, v, a, e)
							}
							if math.Abs(dims.X*dims.Y*dims.Z-out.RemovedVolume-v) > 1e-8 || math.Abs(2*(dims.X*dims.Y+dims.X*dims.Z+dims.Y*dims.Z)+out.SurfaceAreaDelta-a) > 1e-8 {
								t.Fatal("analytic correction")
							}
						}
					}
				}
			}
		}
	}
}
func TestFiniteSkewCrossedBoresMeshAndLimits(t *testing.T) {
	slowtest.Skip(t)
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	dims, c := geom.Vec3{X: 30, Y: 24, Z: 28}, geom.Vec3{X: 15, Y: 12, Z: 14}
	for _, r := range [][2]float64{{4, 2}, {2, 4}} {
		for _, op := range []bool{false, true} {
			out, e := BoxFiniteSkewCrossedBores(context.Background(), f, dims, c, r[0], r[1], 0, 21, 20, op, !op, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			m, e := out.Solid.TessellateFiniteSkewCrossedBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
	}
	old, e := BoxSkewCrossedBores(context.Background(), f, dims, c, 4, 2, 0, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	unchanged, e := BoxFiniteSkewCrossedBores(context.Background(), f, dims, c, 4, 2, 0, 0, 0, false, false, 1e-9)
	if e != nil || !reflect.DeepEqual(old, unchanged.BoxCrossedBoresResult) {
		t.Fatal("changed through", e)
	}
	for _, depth := range []float64{-1, 15, 17, 30, math.NaN(), math.Inf(1)} {
		if _, e := BoxFiniteSkewCrossedBores(context.Background(), f, dims, c, 4, 2, 0, depth, 0, false, false, 1e-9); e == nil {
			t.Fatal("accepted unsafe floor", depth)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxFiniteSkewCrossedBores(ctx, f, dims, c, 4, 2, 0, 21, 20, false, false, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
