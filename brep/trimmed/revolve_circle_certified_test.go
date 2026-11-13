package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestRevolveCircleCertifiedSignedAndPeriodic(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: -3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	for _, angle := range []float64{math.Pi / 2, -math.Pi / 2, 2 * math.Pi} {
		d, e := RevolveCircleCertified(ctx, frame, 3, 1, .5, angle, CertifiedRevolvePolygonOptions{Deflection: .2})
		if e != nil {
			t.Fatal(angle, e)
		}
		if !d.Collision.Complete || len(d.Collision.Contacts) != 0 || d.Mesh.ValidateSolid() != nil ||
			len(d.Mesh.Triangles) != len(d.OriginalFace) || d.FrameDeterminant.Cmp(ratInset(1)) != 0 ||
			d.IdealBoundarySquaredDistance.Cmp(ratInset(.04)) > 0 {
			t.Fatal("incomplete mesh", angle, d.Collision)
		}
		owners := make([]int, len(d.Solid.faces))
		edges := make(map[[2]int][2]int)
		for i, tri := range d.Mesh.Triangles {
			owner := d.OriginalFace[i]
			owners[owner]++
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				k := [2]int{a, b}
				v := edges[k]
				if v[0] == 0 {
					v[0] = owner + 1
				} else {
					v[1] = owner + 1
				}
				edges[k] = v
			}
		}
		nv := d.MeridianSegments
		chi := len(d.Mesh.Vertices) - len(edges) + len(d.Mesh.Triangles)
		if angle == 2*math.Pi {
			if chi != 0 || len(owners) != 1 {
				t.Fatal("periodic display topology", chi, owners)
			}
		} else {
			if chi != 2 || owners[1] != nv || owners[2] != nv {
				t.Fatal("elbow display topology", chi, owners)
			}
			for _, row := range []int{0, d.AngularSegments} {
				for j := 0; j < nv; j++ {
					a, b := row*nv+j, row*nv+(j+1)%nv
					v := edges[[2]int{min(a, b), max(a, b)}]
					if v[0] != 1 || v[1] != 2+row/d.AngularSegments {
						t.Fatal("cap does not share the original rim", row, j, v)
					}
				}
			}
		}
		// Compare against the exact faceted volume formula: the polygonal
		// meridian is swept over equal angular chord stations.
		want := float64(d.AngularSegments*nv) * .25 * 3 / 2 *
			math.Sin(2*math.Pi/float64(nv)) * math.Sin(math.Abs(angle)/float64(d.AngularSegments))
		if got, _ := d.SignedDisplayVolume.Float64(); math.Abs(got-want) > 1e-11*want {
			t.Fatal("signed display volume", angle, got, want)
		}
	}
}

func TestRevolveCircleCertifiedTiltedAndLimits(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: .125, Y: -.25, Z: .5}, geom.Vec3{X: 1}, geom.Vec3{X: 1, Z: 1})
	d, e := RevolveCircleCertified(ctx, f, 2, -.25, .25, -1, CertifiedRevolvePolygonOptions{Deflection: .25})
	if e != nil || d.FrameDeterminant.Sign() <= 0 || !d.Collision.Complete || d.DisplayVertexSquaredError.Sign() == 0 {
		t.Fatal("tilted certificate", e)
	}
	// Midpoint of a known torus parameter triangle uses the same correspondence
	// as the certificate. Test against an independently evaluated formula.
	nu, nv := d.AngularSegments, d.MeridianSegments
	a, b, c := d.Mesh.Vertices[0], d.Mesh.Vertices[nv], d.Mesh.Vertices[nv+1]
	uvU := -1 + 2./(3*float64(nu))
	uvV := 2 * math.Pi / (3 * float64(nv))
	r := 2 + .25*math.Cos(uvV)
	exact := f.ToWorld(geom.Vec3{X: r * math.Cos(uvU), Y: r * math.Sin(uvU), Z: -.25 + .25*math.Sin(uvV)})
	lower, _ := d.IdealBoundarySquaredDistance.Float64()
	if delta := exact.Distance(a.Add(b).Add(c).Scale(1. / 3)); delta*delta > lower {
		t.Fatal("surface correspondence exceeds bound", delta, lower)
	}
	for _, o := range []CertifiedRevolvePolygonOptions{
		{Deflection: .2, MaxSegments: 2},
		{Deflection: .2, MaxVertices: 4},
		{Deflection: .2, MaxTriangles: 4},
		{Deflection: .2, MaxTrigTerms: 1},
	} {
		if got, e := RevolveCircleCertified(ctx, f, 3, 0, .5, 1, o); !errors.Is(e, geom.ErrConvergence) || len(got.Mesh.Vertices) != 0 {
			t.Fatal("budget left partial output", e)
		}
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if _, e := RevolveCircleCertified(cancel, f, 3, 0, .5, 1, CertifiedRevolvePolygonOptions{Deflection: .2}); !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", e)
	}
}
