package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestSphericalLensDilation(t *testing.T) {
	ctx := context.Background()
	a := Ball{Center: geom.Vec3{X: 2, Y: -1, Z: 4}, Radius: 5}
	b := Ball{Center: geom.Vec3{X: 2, Y: -1, Z: -2}, Radius: 5}
	source, err := NewSpherePairSet(ctx, a, b, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	const displacement = .5
	result, history, err := source.DilateSphericalLens(ctx, displacement, 1e-9)
	if err != nil || history.ResultFaces != [2]int{2, 0} || history.BlendFace != 1 || len(result.Faces()) != 3 {
		t.Fatalf("lens dilation = %+v, %v", history, err)
	}
	wantCenter := geom.Vec3{X: 2, Y: -1, Z: 1}
	if history.RimCenter != wantCenter || history.RimRadius != 4 || history.BlendRadius != displacement {
		t.Fatalf("rolling circle = %+v", history)
	}
	faces := result.Faces()
	band, ok := faces[history.BlendFace].Surface.(geom.TorusSurface)
	if !ok {
		t.Fatalf("blend support is %T", faces[history.BlendFace].Surface)
	}
	frame, major, minor := band.Parameters()
	if major != 4 || minor != displacement || frame.Origin() != wantCenter || frame.ZAxis() != (geom.Vec3{Z: -1}) {
		t.Fatalf("blend support radii = %v, %v at %v", major, minor, frame.Origin())
	}
	// The two trimmed joins share the cap normals. The common radial normal
	// on the rolling band is the original lens edge's outward normal cone.
	for i, faceID := range history.ResultFaces {
		sphere := faces[faceID].Surface.(geom.SphereSurface)
		support, radius := sphere.Parameters()
		if support.Origin() != [2]geom.Vec3{a.Center, b.Center}[i] || radius != 5.5 {
			t.Fatalf("sphere %d lost its original center or offset radius", i)
		}
		v := [2]float64{faces[1].V.Max, faces[1].V.Min}[i]
		for _, u := range []float64{0, .3, 2, 5.5} {
			point, err := band.Eval(u, v)
			cap, capErr := sphere.Eval(u, v)
			du, dv, partialsErr := band.Partials(u, v)
			normal, normalErr := du.Cross(dv).Unit()
			if err != nil || capErr != nil || partialsErr != nil || normalErr != nil || point.Distance(cap) > 1e-12 || normal.Distance(cap.Sub(support.Origin()).Scale(1/radius)) > 1e-12 {
				t.Fatalf("join %d at u=%v lost the shared point or normal", i, u)
			}
		}
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, result); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadJSON(bytes.NewReader(native.Bytes()), JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	// The convex parallel-body formula uses original lens volume and area,
	// smooth mean curvature and half the contact circle length times the
	// angle between the original cap normals.
	v0, a0 := 104*math.Pi/3, 40*math.Pi
	curvature := 8*math.Pi + 4*math.Pi*math.Acos(7./25)
	wantVolume := v0 + a0*displacement + curvature*displacement*displacement + 4*math.Pi/3*displacement*displacement*displacement
	wantArea := a0 + 2*curvature*displacement + 4*math.Pi*displacement*displacement
	properties, err := reloaded.MassProperties(ctx, IntegrationOptions{})
	if err != nil || math.Abs(properties.Volume-wantVolume) > 1e-8 || math.Abs(properties.Area-wantArea) > 1e-8 || properties.Centroid.Distance(wantCenter) > 1e-9 {
		t.Fatalf("parallel lens mass=%+v, want V=%v A=%v, err=%v", properties, wantVolume, wantArea, err)
	}
	mesh, err := reloaded.Tessellate(ctx, TessellationOptions{Deflection: .05, AnalyticBound: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := mesh.ValidateSolid(); err != nil {
		t.Fatalf("new blend failed closed tessellation: %v", err)
	}
}

func TestSphericalLensDilationBounds(t *testing.T) {
	ctx := context.Background()
	a := Ball{Radius: 5}
	b := Ball{Center: geom.Vec3{X: 6}, Radius: 5}
	source, err := NewSpherePairSet(ctx, a, b, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	if result, report, err := source.DilateSphericalLens(ctx, 0, 1e-9); err != nil || len(result.Faces()) != 2 || report.BlendFace != -1 {
		t.Fatalf("identity dilation = %+v, %v", report, err)
	}
	for _, tc := range []struct {
		offset float64
		want   error
	}{{.1, geom.ErrUnrepresentable}, {-1, geom.ErrInvalid}} {
		result, report, err := source.DilateSphericalLens(ctx, tc.offset, 1e-9)
		if !errors.Is(err, tc.want) || len(result.Faces()) != 0 || report.BlendFace != -1 || report.Radii[0] != nil {
			t.Fatalf("distance %v published partial data: %+v, %v", tc.offset, report, err)
		}
	}
	a.Radius, b.Radius, b.Center.X = 2, 2, 2
	irrational, err := NewSpherePairSet(ctx, a, b, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := irrational.DilateSphericalLens(ctx, .5, 1e-9); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatalf("irrational rim was rounded into a model support: %v", err)
	}
	if r, err := irrational.ClassifyDilatedSphericalLens(ctx, geom.Vec3{X: 1, Z: 2}, .5, geom.SphereFullBoundaryDistanceOptions{}); err != nil || r.Location != geom.BoreInside {
		t.Fatalf("exact classification with a nonrepresentable rim: %+v %v", r, err)
	}
}

func TestSphericalLensDilationBeyondRingRadius(t *testing.T) {
	ctx := context.Background()
	source, err := NewSpherePairSet(ctx, Ball{Radius: 5}, Ball{Center: geom.Vec3{Z: 6}, Radius: 5}, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []float64{4, 12} {
		dilated, history, err := source.DilateSphericalLens(ctx, offset, 1e-9)
		if err != nil || history.RimRadius != 4 || history.BlendRadius != offset {
			t.Fatalf("offset %v: %+v, %v", offset, history, err)
		}
		if _, ok := dilated.Faces()[history.BlendFace].Surface.(geom.OuterTorusSurface); !ok {
			t.Fatalf("offset %v uses %T", offset, dilated.Faces()[history.BlendFace].Surface)
		}
		var data bytes.Buffer
		if err := WriteJSON(&data, dilated); err != nil {
			t.Fatal(err)
		}
		s, err := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
		if err != nil {
			t.Fatal(err)
		}
		v0, a0 := 104*math.Pi/3, 40*math.Pi
		curvature := 8*math.Pi + 4*math.Pi*math.Acos(7./25)
		volume := v0 + a0*offset + curvature*offset*offset + 4*math.Pi/3*offset*offset*offset
		area := a0 + 2*curvature*offset + 4*math.Pi*offset*offset
		mass, err := s.MassProperties(ctx, IntegrationOptions{})
		if err != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 || mass.Centroid.Distance(geom.Vec3{Z: 3}) > 1e-9 {
			t.Fatalf("offset %v mass=%+v want %g, %g; %v", offset, mass, volume, area, err)
		}
		bounds, err := s.Bounds()
		wantMin, wantMax := (geom.Vec3{X: -4 - offset, Y: -4 - offset, Z: 1 - offset}), (geom.Vec3{X: 4 + offset, Y: 4 + offset, Z: 5 + offset})
		if err != nil || !bounds.Contains(wantMin) || !bounds.Contains(wantMax) || bounds.Min.Distance(wantMin) > 2e-9 || bounds.Max.Distance(wantMax) > 2e-9 {
			t.Fatalf("offset %v bounds=%+v; %v", offset, bounds, err)
		}
		hits, err := s.Ray(geom.Vec3{X: -20, Z: 3}, geom.Vec3{X: 1}, 40, 1e-8)
		if err != nil || len(hits) != 2 || math.Abs(hits[0].Point.X+4+offset) > 1e-8 || math.Abs(hits[1].Point.X-4-offset) > 1e-8 {
			t.Fatalf("offset %v ray=%+v; %v", offset, hits, err)
		}
		clip, err := source.ClipDilatedSphericalLensLine(ctx, geom.Vec3{X: -20, Z: 3}, geom.Vec3{X: 1}, 0, 40, offset, geom.ToroidLineOptions{ParameterTolerance: 1e-9})
		both := geom.SpherePairFirst | geom.SpherePairSecond
		if err != nil || clip.Interval == nil || clip.Interval.StartBoundary != both || clip.Interval.EndBoundary != both {
			t.Fatalf("offset %v exact clip=%+v; %v", offset, clip, err)
		}
		for i, interval := range []geom.ExactParameterInterval{clip.Interval.StartBounds, clip.Interval.EndBounds} {
			lo, _ := interval.Lower.Float64()
			hi, _ := interval.Upper.Float64()
			if hits[i].Distance < lo-1e-9 || hits[i].Distance > hi+1e-9 {
				t.Fatalf("ray and exact interval differ: %+v, %+v", hits[i], interval)
			}
		}
		m, err := s.Tessellate(ctx, TessellationOptions{Deflection: .15, AnalyticBound: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAsymmetricSphericalLensDilation(t *testing.T) {
	ctx := context.Background()
	a := Ball{Center: geom.Vec3{X: -4, Y: 1, Z: 2}, Radius: 13}
	b := Ball{Center: geom.Vec3{X: 10, Y: 1, Z: 2}, Radius: 15}
	source, err := NewSpherePairSet(ctx, a, b, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	const d = .25
	result, history, err := source.DilateSphericalLens(ctx, d, 1e-9)
	if err != nil || history.RimCenter != (geom.Vec3{X: 1, Y: 1, Z: 2}) || history.RimRadius != 12 {
		t.Fatalf("asymmetric rolling circle = %+v, %v", history, err)
	}
	// The cap heights are 8 and 6. The original normals have dot
	// product 33/65 on the common rim, regardless of azimuth.
	v0 := math.Pi * (64*(13-8./3) + 36*(15-6./3))
	a0 := 388 * math.Pi
	curvature := 28*math.Pi + 12*math.Pi*math.Acos(33./65)
	volume := v0 + a0*d + curvature*d*d + 4*math.Pi/3*d*d*d
	area := a0 + 2*curvature*d + 4*math.Pi*d*d
	p, err := result.MassProperties(ctx, IntegrationOptions{})
	if err != nil || math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
		t.Fatalf("asymmetric lens mass=%+v, want V=%v A=%v, %v", p, volume, area, err)
	}
}
