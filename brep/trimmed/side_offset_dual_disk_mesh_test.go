//go:build research

package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestOffsetSideOpenDualDiskCertifiedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, depth := range []float64{3, 6} {
		for _, permuted := range []bool{false, true} {
			t.Run(fmt.Sprintf("depth%g/permuted%v", depth, permuted), func(t *testing.T) {
				x, z := geom.Vec3{X: 1}, geom.Vec3{Z: 1}
				if permuted {
					x, z = geom.Vec3{Y: 1}, geom.Vec3{X: -1}
				}
				frame, e := geom.NewFrame(geom.Vec3{X: 10, Y: 20, Z: 30}, x, z)
				if e != nil {
					t.Fatal(e)
				}
				ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
				if permuted {
					ds[0], ds[1] = ds[1], ds[0]
				}
				s, e := CutOffsetSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, ds, 6, depth, 1e-10, trimvalidate.Options{})
				if e != nil {
					t.Fatal(e)
				}
				o := SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2}
				r, e := s.TessellateCertified(ctx, o)
				if e != nil {
					t.Fatal(e)
				}
				if e = r.Mesh.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				legacy, e := collision.SelfIntersections(r.Mesh, collision.Options{})
				if e != nil {
					t.Fatal(e)
				}
				if !legacy.Complete || len(legacy.Contacts) != 0 {
					t.Fatal("independent exact contact check", legacy)
				}
				prim := func(x, r float64) float64 { return (x*math.Sqrt(r*r-x*x) + r*r*math.Asin(x/r)) / 2 }
				removed := -7.5*(239./8-75./8) + prim(2, 65./8) - prim(-25./8, 65./8) + prim(63./8, 87./8) - prim(-7.5, 87./8)
				want := 2400 - depth*removed
				v, _ := r.DisplayVolume.Float64()
				// Circular segment loss is bounded by r² theta³/12 per chord.
				// Sum theta³ <= max(tan² theta) sum theta for these short monotone arcs.
				tangent, _ := r.SquaredTurnTangent.Float64()
				weighted := math.Pow(65./8, 2)*(math.Asin(16./65)+math.Asin(25./65)) + math.Pow(87./8, 2)*(math.Asin(63./87)+math.Asin(60./87))
				exactVolume := new(big.Rat)
				for _, face := range r.Network.Faces {
					if face.RectangularGrid {
						for _, uv := range face.ExactUV {
							if uv[1].Sign() != 0 && uv[1].Cmp(big.NewRat(1, 1)) != 0 {
								t.Fatal("fixture requires axial end rows")
							}
						}
					}
					for _, tri := range face.Mesh.Triangles {
						exactVolume.Add(exactVolume, ddDot(face.ExactPoints[tri[0]], ddCross(face.ExactPoints[tri[1]], face.ExactPoints[tri[2]])))
					}
				}
				exactVolume.Quo(exactVolume, big.NewRat(6, 1))
				rounding, _ := new(big.Rat).Abs(new(big.Rat).Sub(r.DisplayVolume, exactVolume)).Float64()
				allowance := depth*tangent*weighted/12 + math.Nextafter(rounding, math.Inf(1))
				if v < want-1e-8 || v > want+allowance+1e-8 {
					t.Fatal("segment volume bound", v, want, allowance)
				}
				var data bytes.Buffer
				if e = WriteJSON(&data, s.Solid); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(ctx, &data, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				q, e := loaded.TessellateDualCircularWallCertified(ctx, o)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(r.Mesh, q.Mesh) || r.SquaredDistance.Cmp(q.SquaredDistance) != 0 || r.SquaredFacetTangent.Cmp(q.SquaredFacetTangent) != 0 {
					t.Fatal("reload altered certificate")
				}
				t.Log(len(r.Mesh.Triangles), v, want, allowance)
			})
		}
	}
}
