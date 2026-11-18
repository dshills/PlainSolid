package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

func TestSideOpenDualDiskCertifiedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	for caseID, depth := range []float64{3, 6, 3} {
		if caseID == 2 {
			frame, _ = geom.NewFrame(geom.Vec3{X: 7, Y: -2, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
		}
		s, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		start := time.Now()
		r, e := s.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2})
		if e != nil {
			t.Fatal(depth, e)
		}
		t.Log(depth, "intervals", r.RimIntervals, "triangles", len(r.Mesh.Triangles), "time", time.Since(start), "turn²", r.SquaredTurnTangent.FloatString(8), "facet²", r.SquaredFacetTangent.FloatString(8))
		if r.RimIntervals > 64 || len(r.Mesh.Triangles) > 2000 {
			t.Fatal("unexpected refinement")
		}
		d, _ := r.SquaredDistance.Float64()
		a, _ := r.SquaredTurnTangent.Float64()
		f, _ := r.SquaredFacetTangent.Float64()
		if d > .05*.05 || a > .2*.2 || f > .1*.1 {
			t.Fatal(d, a, f)
		}
		exactVolume := checkDualDiskSegmentVolume(t, r, depth)
		if caseID == 0 {
			previous := exactVolume
			for _, deflection := range []float64{.0125, .0035} {
				fine, e := s.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: deflection, AngularTurn: .2})
				if e != nil {
					t.Fatal(e)
				}
				next := checkDualDiskSegmentVolume(t, fine, depth)
				if next.Cmp(previous) >= 0 {
					t.Fatal("inscribed cutter volume did not converge", deflection)
				}
				previous = next
			}
		}
		// Independent floating normal samples verify the exact bound against the
		// original surface differential, including interior UV values.
		for fi, face := range r.Network.Faces {
			for _, tri := range face.Mesh.Triangles {
				a, b, c := face.Mesh.Vertices[tri[0]], face.Mesh.Vertices[tri[1]], face.Mesh.Vertices[tri[2]]
				normal, _ := b.Sub(a).Cross(c.Sub(a)).Unit()
				for _, weights := range [][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {.2, .3, .5}} {
					var u, v float64
					for j, id := range tri {
						x, _ := face.ExactUV[id][0].Float64()
						y, _ := face.ExactUV[id][1].Float64()
						u += weights[j] * x
						v += weights[j] * y
					}
					expected, e := s.Solid.faces[fi].Surface.(*geom.BSplineSurface).Normal(u, v)
					if e != nil {
						t.Fatal(e)
					}
					if s.Solid.faces[fi].Reversed {
						expected = expected.Scale(-1)
					}
					if normal.Dot(expected) < math.Cos(.1)-1e-12 {
						t.Fatal("independent normal bound", fi, normal.Dot(expected))
					}
				}
			}
		}
		if e = r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for _, o := range []SideOpenDualDiskMeshOptions{{Deflection: .05, AngularTurn: .2, MaxRimIntervals: 1}, {Deflection: .05, AngularTurn: .2, MaxTriangles: 1}, {Deflection: 100, AngularTurn: .01, MaxRimIntervals: 2}} {
			bad, e := s.TessellateCertified(ctx, o)
			if e == nil || len(bad.Mesh.Vertices) != 0 {
				t.Fatal("budget returned output", e)
			}
		}
		bad := s
		bad.WallFaces[0] = []int{0}
		if _, e = bad.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2}); e == nil {
			t.Fatal("unverified metadata accepted")
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e = s.TessellateCertified(cancelled, SideOpenDualDiskMeshOptions{Deflection: .05, AngularTurn: .2}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
}

func TestSideOpenDualDiskApplicationTolerances(t *testing.T) {
	slowtest.Skip(t)
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	for _, depth := range []float64{3, 6} {
		s, e := CutSideOpenDualDisks(context.Background(), frame, geom.Vec2{Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, 6, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		for _, o := range []SideOpenDualDiskMeshOptions{{Deflection: .0035, AngularTurn: .08}, {Deflection: .1, AngularTurn: .05}} {
			start := time.Now()
			r, e := s.TessellateCertified(context.Background(), o)
			if e != nil {
				t.Fatal(depth, o, e)
			}
			t.Log(depth, o, "intervals", r.RimIntervals, "triangles", len(r.Mesh.Triangles), "time", time.Since(start), "pairs", r.DisplayCollision.BroadPhasePairs)
			d, _ := r.SquaredDistance.Float64()
			a, _ := r.SquaredTurnTangent.Float64()
			f, _ := r.SquaredFacetTangent.Float64()
			if d > o.Deflection*o.Deflection || a > o.AngularTurn*o.AngularTurn || f > o.AngularTurn*o.AngularTurn/4 {
				t.Fatal(d, a, f)
			}
		}
	}
}

// This independent fixture-specific mass check is NOT part of the public
// distance/angle/embedding contract. A circular segment has area
// r²(theta-sin(theta))/2 <= r²*theta³/12. Summing and using
// theta² <= max(tan²(theta)) gives a volume error bound from the actual rim
// partition. Display rounding is accounted for by exact determinant integration.
func checkDualDiskSegmentVolume(t *testing.T, r CertifiedSideOpenDualDiskMesh, depth float64) *big.Rat {
	t.Helper()
	exactVolume := new(big.Rat)
	walls := 0
	for _, face := range r.Network.Faces {
		if face.RectangularGrid {
			walls++
			for _, uv := range face.ExactUV {
				if uv[1].Sign() != 0 && uv[1].Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("mass fixture requires two axial rim rows")
				}
			}
		}
	}
	if walls != 2 {
		t.Fatal("mass fixture circular wall ownership", walls)
	}
	for _, face := range r.Network.Faces {
		for _, tri := range face.Mesh.Triangles {
			a, b, c := face.ExactPoints[tri[0]], face.ExactPoints[tri[1]], face.ExactPoints[tri[2]]
			exactVolume.Add(exactVolume, ddDot(a, ddCross(b, c)))
		}
	}
	exactVolume.Quo(exactVolume, big.NewRat(6, 1))
	rounding := new(big.Rat).Sub(r.DisplayVolume, exactVolume)
	rounding.Abs(rounding)
	allowance, _ := rounding.Float64()
	allowance = math.Nextafter(allowance, math.Inf(1))
	v, _ := r.DisplayVolume.Float64()
	want := 360 - depth*(6.25*(math.Pi-math.Acos(.6))+3)
	turn, _ := r.SquaredTurnTangent.Float64()
	upper := depth*6.25*turn*(2*(math.Pi-math.Acos(.6)))/12 + allowance
	if v-want < -allowance-1e-10 || v-want > upper+1e-10 {
		t.Fatal("circle-segment mass enclosure", v, want, upper, allowance)
	}
	return exactVolume
}
