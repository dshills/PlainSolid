package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"plainsolid/geom"
)

func TestExactWrappedFrustumBoreMass(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	cylinder := func(c geom.Vec3, r, h float64) *geom.WrappedSurfacePatch {
		t.Helper()
		w, e := geom.NewCanonicalWrappedCylinder(ctx, c, r, h, o)
		if e != nil {
			t.Fatal(e)
		}
		return w
	}
	check := func(x *big.Rat, numerator, denominator int64) {
		t.Helper()
		q := big.NewRat(numerator, denominator)
		if x == nil || x.Cmp(q) != 0 {
			t.Fatal("independent exact mass coefficient", x, q)
		}
	}
	// Closed-form elementary cylinder moments make an independent tensor
	// check. Through hole center=(1/2,1/2); stock center=(0,0), heights 0..4.
	stock := cylinder(geom.Vec3{}, 3, 4)
	inner := cylinder(geom.Vec3{X: .5, Y: .5}, 1, 4)
	m, e := MeasureWrappedEccentricFrustumBore(ctx, stock, inner, o)
	if e != nil || len(m.Area) != 4 {
		t.Fatal(m.Area, e)
	}
	check(m.VolumePi, 32, 1)
	check(m.Centroid[0], -1, 16)
	check(m.Centroid[1], -1, 16)
	check(m.Centroid[2], 2, 1)
	check(m.SecondMomentPi[0][0], 79, 1)
	check(m.SecondMomentPi[0][1], -1, 1)
	check(m.SecondMomentPi[0][2], -4, 1)
	check(m.SecondMomentPi[2][2], 512, 3)
	check(m.InertiaAtCentroidPi[0][0], 2917, 24)
	check(m.InertiaAtCentroidPi[2][2], 631, 4)
	check(m.InertiaAtCentroidPi[0][1], 9, 8)
	check(m.InertiaAtCentroidPi[1][0], 9, 8)
	check(m.InertiaAtCentroidPi[0][2], 0, 1)
	for i, a := range m.Area {
		if a.Face != i {
			t.Fatal("lost analytic boundary owner", a)
		}
		check(a.Coefficient, [4]int64{6, 8, 8, 2}[i], 1)
		check(a.Radicand, [4]int64{16, 1, 1, 16}[i], 1)
	}
	// The slant is exactly five for the stock of radii 2 and 5 over
	// height four. A short eccentric cavity keeps both full outer caps
	// and both inner disks. Independent integrals give V=49π, Mz=126π.
	stock, _, e = geom.DraftWrappedFrustum(ctx, cylinder(geom.Vec3{}, 2, 4), 0, 3, o)
	if e != nil {
		t.Fatal(e)
	}
	inner = cylinder(geom.Vec3{X: .25, Y: .25, Z: .5}, 1, 3)
	body, h, e := NewWrappedEccentricCircularFrustumBore(ctx, stock, inner, 1e-8, o)
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	m, e = body.MeasureCanonicalWrappedFrustumBore(ctx, o)
	if e != nil || len(m.Area) != 6 {
		t.Fatal(m.Area, e)
	}
	check(m.VolumePi, 49, 1)
	check(m.Centroid[0], -3, 196)
	check(m.Centroid[2], 18, 7)
	owners := [6]int{h.OuterFaces[0], h.OuterFaces[1], h.OuterFaces[2], h.InnerFaces[0], h.InnerFaces[1], h.InnerFaces[2]}
	for i, a := range m.Area {
		if a.Face != len(faces)-1-owners[i] {
			t.Fatal("area owner must retain receiver native index", i, a.Face)
		}
		check(a.Coefficient, [6]int64{7, 4, 25, 2, 1, 1}[i], 1)
		check(a.Radicand, [6]int64{25, 1, 1, 9, 1, 1}[i], 1)
	}
}

func TestOCCTWrappedFrustumBoreMass(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_BORE_DISTANCE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_FRUSTUM_BORE_DISTANCE to the optional OCCT executable")
	}
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	raw, err := exec.CommandContext(ctx, program, "mass").CombinedOutput()
	if err != nil {
		t.Fatalf("independent OCCT mass properties: %v: %s", err, raw)
	}
	var ref [][]float64
	if err = json.Unmarshal(raw, &ref); err != nil || len(ref) != 2 {
		t.Fatal("invalid OCCT mass record", string(raw), err)
	}
	o := geom.SurfacePatchOptions{}
	for i, data := range ref {
		if len(data) != 14 {
			t.Fatal("malformed independent mass row", data)
		}
		var rx, h, z, offset float64 = 3, 4, 0, .5
		if i == 1 {
			rx, h, z, offset = 2, 3, .5, .25
		}
		stock, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, rx, 4, o)
		if e != nil {
			t.Fatal(e)
		}
		if i == 1 {
			stock, _, e = geom.DraftWrappedFrustum(ctx, stock, 0, 3, o)
			if e != nil {
				t.Fatal(e)
			}
		}
		tool, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: offset, Y: offset, Z: z}, 1, h, o)
		if e != nil {
			t.Fatal(e)
		}
		m, e := MeasureWrappedEccentricFrustumBore(ctx, stock, tool, o)
		if e != nil {
			t.Fatal(e)
		}
		f := func(r *big.Rat) float64 { x, _ := r.Float64(); return x }
		area := 0.0
		for _, term := range m.Area {
			area += f(term.Coefficient) * math.Sqrt(f(term.Radicand)) * math.Pi
		}
		got := []float64{f(m.VolumePi) * math.Pi, area, f(m.Centroid[0]), f(m.Centroid[1]), f(m.Centroid[2])}
		for _, row := range m.InertiaAtCentroidPi {
			for _, x := range row {
				got = append(got, f(x)*math.Pi)
			}
		}
		for j, exact := range got {
			if math.Abs(exact-data[j]) > 1e-7*max(1, math.Abs(exact)) {
				t.Fatal("exact model properties disagree with independent OCCT", i, j, exact, data[j])
			}
		}
	}
}
