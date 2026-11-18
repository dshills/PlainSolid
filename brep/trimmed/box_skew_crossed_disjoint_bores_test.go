package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func companionSites() []CardinalThroughBore {
	return []CardinalThroughBore{{1, geom.Vec3{X: 32, Z: 8}, 2}, {0, geom.Vec3{Y: 8, Z: 32}, 1}, {2, geom.Vec3{X: 8, Y: 32}, 1}, {2, geom.Vec3{X: 4, Y: 4}, 1}}
}
func TestSkewCrossedCompanions(t *testing.T) {
	slowtest.Skip(t)
	dims, c := geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 20, Z: 20}
	ctx := context.Background()
	for _, r := range [][2]float64{{4, 2}, {2, 4}} {
		for _, offset := range []float64{0, .5} {
			for k, depths := range [][2]float64{{0, 0}, {30, 0}, {0, 30}, {30, 30}} {
				for _, op := range []bool{false, true} {
					normal := geom.Vec3{Z: 1}
					if op {
						normal = geom.Vec3{X: -1}
					}
					f, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: -4, Z: 7}, normal)
					original, e := BoxFiniteSkewCrossedBores(ctx, f, dims, c, r[0], r[1], offset, depths[0], depths[1], op, !op, 1e-9)
					if e != nil {
						t.Fatal(e)
					}
					sites := companionSites()[:2]
					if k%2 == 0 {
						sites = sites[:1]
					}
					input := append([]CardinalThroughBore(nil), sites...)
					out, e := BoxFiniteSkewCrossedBoresWithDisjoint(ctx, f, dims, c, r[0], r[1], offset, depths[0], depths[1], op, !op, sites, 1e-9)
					if e != nil {
						t.Fatal(r, offset, depths, op, e)
					}
					if !reflect.DeepEqual(input, sites) {
						t.Fatal("mutated sites")
					}
					if out.XFloorFace != original.XFloorFace || out.ZFloorFace != original.ZFloorFace || len(out.Solid.faces) != len(original.Solid.faces)+4*len(sites) {
						t.Fatal("changed core face IDs")
					}
					if !reflect.DeepEqual(out.Solid.vertices[:len(original.Solid.vertices)], original.Solid.vertices) || !reflect.DeepEqual(out.Solid.edges[:len(original.Solid.edges)], original.Solid.edges) || !reflect.DeepEqual(out.Solid.faces[6:len(original.Solid.faces)], original.Solid.faces[6:]) {
						t.Fatal("changed core geometry")
					}
					removed, delta := original.RemovedVolume, original.SurfaceAreaDelta
					for _, s := range sites {
						removed += math.Pi * s.Radius * s.Radius * 40
						delta += 2 * math.Pi * s.Radius * (40 - s.Radius)
					}
					if math.Abs(out.RemovedVolume-removed) > 1e-9 || math.Abs(out.SurfaceAreaDelta-delta) > 1e-9 {
						t.Fatal("measure correction")
					}
					mass, e := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
					if e != nil || math.Abs(mass.Volume-(64000-removed)) > 1e-6 || math.Abs(mass.Area-(9600+delta)) > 1e-6 {
						t.Fatal(mass, removed, delta, e)
					}
					for i, j := 0, len(sites)-1; i < j; i, j = i+1, j-1 {
						sites[i], sites[j] = sites[j], sites[i]
					}
					reversed, e := BoxFiniteSkewCrossedBoresWithDisjoint(ctx, f, dims, c, r[0], r[1], offset, depths[0], depths[1], op, !op, sites, 1e-9)
					if e != nil || !reflect.DeepEqual(out, reversed) {
						t.Fatal("order changed topology", e)
					}
				}
			}
		}
	}
}
func TestSkewCrossedCompanionsMeshAndReject(t *testing.T) {
	slowtest.Skip(t)
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	dims, c := geom.Vec3{X: 40, Y: 40, Z: 40}, geom.Vec3{X: 20, Y: 20, Z: 20}
	ctx := context.Background()

	for _, r := range [][2]float64{{4, 2}, {2, 4}} {
		for _, offset := range []float64{0, .5} {
			for _, depths := range [][2]float64{{0, 0}, {15, 15}} {
				out, e := BoxFiniteSkewCrossedBoresWithDisjoint(ctx, f, geom.Vec3{X: 20, Y: 24, Z: 18}, geom.Vec3{X: 8.25, Y: 12.5, Z: 9}, r[0], r[1], offset, depths[0], depths[1], false, true, []CardinalThroughBore{{1, geom.Vec3{X: 16, Z: 3}, 1}, {1, geom.Vec3{X: 3, Z: 15}, 1}}, 1e-9)
				if e != nil {
					t.Fatal(e)
				}
				m, e := out.Solid.TessellateSkewCrossedWithDisjoint(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 200000})
				if e != nil {
					t.Fatal(r, offset, depths, e)
				}
				if e = m.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				candidate := m
				candidate.Vertices = append([]geom.Vec3(nil), m.Vertices...)
				for i, p := range candidate.Vertices {
					candidate.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
				}
				if e = candidate.ValidateSolid(); e != nil {
					t.Fatal("float32", e)
				}
				report, e := collision.SelfIntersectionsBVHContext(ctx, candidate, collision.Options{})
				if e != nil || !report.Complete || len(report.Contacts) != 0 {
					t.Fatal("float32 embedding", e, report)
				}
				if _, e = out.Solid.TessellateSkewCrossedWithDisjoint(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .05, MaxSubdivisions: 128, MaxTriangles: 2}); e == nil {
					t.Fatal("small mesh budget ignored")
				}
				t.Logf("radii%v offset%g depths%v triangles%d", r, offset, depths, len(m.Triangles))
			}
		}
	}

	good := companionSites()[0]
	for _, sites := range [][]CardinalThroughBore{nil, {good, good}, make([]CardinalThroughBore, 5), {{0, geom.Vec3{Y: 20, Z: 20}, 1}}, {{0, geom.Vec3{Y: 25, Z: 20}, 1}}, {{-1, geom.Vec3{}, 1}}, {{1, geom.Vec3{X: 32, Y: 1, Z: 8}, 2}}, {{1, geom.Vec3{X: 32, Z: 8}, math.NaN()}}} {
		if _, e := BoxSkewCrossedBoresWithDisjoint(ctx, f, dims, c, 4, 2, 0, sites, 1e-9); e == nil {
			t.Fatal("accepted outside proof", sites)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := BoxSkewCrossedBoresWithDisjoint(canceled, f, dims, c, 4, 2, 0, []CardinalThroughBore{good}, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
