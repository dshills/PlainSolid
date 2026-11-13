package parametric

import (
	"context"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFiniteEccentricCylinderShoulderUnion(t *testing.T) {
	ctx := context.Background()
	wide := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 1, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 4, Min: 0, Max: 2}
	for _, lower := range []bool{false, true} {
		for _, touch := range []bool{false, true} {
			for _, swap := range []bool{false, true} {
				narrow := geom.RawCylinder{Origin: geom.Vec3{X: 3, Y: 1}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -1.5, Max: .5}
				shoulder, far, outerEnd := -1., -3., 3.
				if lower {
					narrow.Min, narrow.Max = .5, 2.5
					shoulder, far, outerEnd = 3, 5, -1
				}
				if touch {
					if lower {
						narrow.Min = 1.5
					} else {
						narrow.Max = -.5
					}
				}
				a, b, wideIndex := wide, narrow, 0
				if swap {
					a, b, wideIndex = b, a, 1
				}
				r, err := FiniteCylinderPairUnion(ctx, a, b, 1e-9)
				if err != nil || len(r.Solid.Faces()) != 20 || len(r.WallFaces[0]) != 4 || len(r.WallFaces[1]) != 4 {
					t.Fatal(lower, touch, swap, "retained shoulder", err)
				}
				if touch && (r.CommonVolumeSquared == nil || r.CommonVolumeSquared.Sign() != 0) || !touch && (r.CommonVolumePiCoefficient == nil || r.CommonVolumePiCoefficient.RatString() != "8") {
					t.Fatal("axial common proof")
				}
				for i, item := range [][2][]int{{r.LowerCapFaces[wideIndex], r.UpperCapFaces[wideIndex]}, {r.LowerCapFaces[1-wideIndex], r.UpperCapFaces[1-wideIndex]}} {
					for j, faces := range item {
						want := [2][2]float64{{outerEnd, shoulder}, {far, math.NaN()}}
						if lower {
							want = [2][2]float64{{shoulder, outerEnd}, {math.NaN(), far}}
						}
						if math.IsNaN(want[i][j]) && len(faces) != 0 || !math.IsNaN(want[i][j]) && len(faces) != 4 {
							t.Fatal("raw cap ownership", i, j, faces)
						}
						for _, id := range faces {
							f := r.Solid.Faces()[id]
							p, err := f.Surface.Eval(f.U.Min, f.V.Max)
							if err != nil || p.Z != want[i][j] {
								t.Fatal("original cap plane", p, err)
							}
						}
					}
				}
				m, err := r.Solid.MassProperties(ctx, IntegrationOptions{})
				center := geom.Vec3{X: 19. / 9, Y: 1, Z: (64 + 4*(far+shoulder)) / 72}
				if err != nil || math.Abs(m.Volume-72*math.Pi) > 1e-8 || math.Abs(m.Area-72*math.Pi) > 1e-8 || m.Centroid.Distance(center) > 1e-9 {
					t.Fatal("mass and shoulder area", m, err)
				}
				mesh, err := r.Solid.Tessellate(ctx, TessellationOptions{Deflection: .1})
				if err != nil || mesh.ValidateSolid() != nil {
					t.Fatal("closed mesh", err)
				}
			}
		}
	}
	tangent := geom.RawCylinder{Origin: geom.Vec3{X: 4, Y: 1}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -1.5, Max: .5}
	if r, err := FiniteCylinderPairUnion(ctx, wide, tangent, 1e-9); err == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("tangent shoulder accepted", err)
	}
}
