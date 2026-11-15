package parametric

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"plainsolid/geom"
)

func sphericalOffsetFixture(t *testing.T) SpherePairSet {
	t.Helper()
	outer := Ball{Center: geom.Vec3{X: 2, Y: -3, Z: .5}, Radius: 4}
	inner := Ball{Center: geom.Vec3{X: 3, Y: -3, Z: .5}, Radius: 1}
	set, err := NewSpherePairSet(context.Background(), outer, inner, Difference, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestSphericalCavityOffset(t *testing.T) {
	ctx := context.Background()
	source := sphericalOffsetFixture(t)
	for _, tc := range []struct {
		distance           float64
		outer, inner       float64
		volume, area       float64
		faces              int
		transverse, inward bool
	}{
		{.25, 4.25, .75, 4 * math.Pi / 3 * (4.25*4.25*4.25 - .75*.75*.75), 4 * math.Pi * (4.25*4.25 + .75*.75), 2, false, true},
		{-.25, 3.75, 1.25, 4 * math.Pi / 3 * (3.75*3.75*3.75 - 1.25*1.25*1.25), 4 * math.Pi * (3.75*3.75 + 1.25*1.25), 2, false, true},
		{1, 5, 0, 4 * math.Pi / 3 * 125, 4 * math.Pi * 25, 1, false, false},
		{1.5, 5.5, -.5, 4 * math.Pi / 3 * 5.5 * 5.5 * 5.5, 4 * math.Pi * 5.5 * 5.5, 1, false, false},
		{-2, 2, 3, 0, 0, 0, false, false},
		{-4, 0, 5, 0, 0, 0, false, false},
		// The overlap has spherical cap heights 1 and 3 at x=7/4.
		{-1.25, 2.75, 2.25, math.Pi * 225 / 16, math.Pi * 38.25, 2, true, true},
	} {
		t.Run(strconv.FormatFloat(tc.distance, 'g', -1, 64), func(t *testing.T) {
			solid, report, err := source.OffsetSphericalCavity(ctx, tc.distance, 1e-9)
			if err != nil || len(solid.Faces()) != tc.faces || report.SourceFaces != [2]int{0, 1} {
				t.Fatalf("result faces=%d, report=%+v, error=%v", len(solid.Faces()), report, err)
			}
			if report.Radii[0].Cmp(new(big.Rat).SetFloat64(tc.outer)) != 0 || report.Radii[1].Cmp(new(big.Rat).SetFloat64(tc.inner)) != 0 {
				t.Fatalf("offset radii %v do not match represented design", report.Radii)
			}
			if tc.faces == 0 {
				if report.ResultFaces != [2]int{-1, -1} {
					t.Fatalf("empty result retained a face: %+v", report)
				}
				return
			}
			if report.ResultFaces[0] < 0 || (report.ResultFaces[1] >= 0) != tc.inward {
				t.Fatalf("invalid retained owners: %+v", report)
			}
			for i, id := range report.ResultFaces {
				if id < 0 {
					continue
				}
				face := solid.Faces()[id]
				support, ok := face.Surface.(geom.SphereSurface)
				if !ok || face.Reversed != (i == 1) || (face.V.Min != -math.Pi/2 || face.V.Max != math.Pi/2) != tc.transverse {
					t.Fatalf("support owner %d invalid: %+v", i, face)
				}
				frame, radius := support.Parameters()
				if frame.Origin() != (geom.Vec3{X: float64(i + 2), Y: -3, Z: .5}) || radius != [2]float64{tc.outer, tc.inner}[i] {
					t.Fatalf("face %d changed its original sphere center or radius", i)
				}
			}
			var data bytes.Buffer
			if err := WriteJSON(&data, solid); err != nil {
				t.Fatal(err)
			}
			reloaded, err := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
			if err != nil {
				t.Fatal(err)
			}
			properties, err := reloaded.MassProperties(ctx, IntegrationOptions{})
			if err != nil || math.Abs(properties.Volume-tc.volume) > 1e-7 || math.Abs(properties.Area-tc.area) > 1e-7 {
				t.Fatalf("reloaded mass %v, want V=%v A=%v (%v)", properties, tc.volume, tc.area, err)
			}
		})
	}
}

func TestSphericalCavityOffsetAtomicLimits(t *testing.T) {
	source := sphericalOffsetFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		distance float64
		want     error
	}{
		{-1, geom.ErrDomain}, // expansion reaches internal tangency
		{.1, geom.ErrUnrepresentable},
		{math.Inf(1), geom.ErrInvalid},
	} {
		solid, report, err := source.OffsetSphericalCavity(ctx, tc.distance, 1e-9)
		if !errors.Is(err, tc.want) || len(solid.Faces()) != 0 || report.ResultFaces != [2]int{-1, -1} || report.Radii[0] != nil {
			t.Fatalf("offset %v exposed partial state: %+v, %v", tc.distance, report, err)
		}
	}
	zero, history, err := source.OffsetSphericalCavity(ctx, 0, 1e-9)
	if err != nil || history.SourceFaces != history.ResultFaces || len(zero.Faces()) != 2 || zero.Faces()[0].Surface != source.Solid().Faces()[0].Surface {
		t.Fatalf("identity changed the source ownership: %+v, %v", history, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := source.OffsetSphericalCavity(canceled, .25, 1e-9); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled offset = %v", err)
	}
	union, err := NewSpherePairSet(ctx, Ball{Radius: 1}, Ball{Center: geom.Vec3{X: 3}, Radius: 1}, Union, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := union.OffsetSphericalCavity(ctx, .25, 1e-9); !errors.Is(err, geom.ErrDomain) {
		t.Fatalf("union was accepted as a cavity: %v", err)
	}
}

func TestSphericalCavityOffsetOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET to the optional OCCT comparator")
	}
	ctx := context.Background()
	source := sphericalOffsetFixture(t)
	for _, row := range []struct {
		mode     string
		distance float64
	}{
		{"simple-offset", .25}, {"simple-offset", -.25}, {"supports", 1}, {"supports", -1.25}, {"supports", -2},
	} {
		solid, _, err := source.OffsetSphericalCavity(ctx, row.distance, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		n := 1
		if row.distance == -2 {
			n = 0
		}
		checkSphereOffsetAgainstOCCT(t, bin, row.mode, row.distance, n, solid)
	}
}

func checkSphereOffsetAgainstOCCT(t *testing.T, bin, mode string, distance float64, wantSolids int, solid Solid) {
	t.Helper()
	str := strconv.FormatFloat(distance, 'g', -1, 64)
	raw, err := exec.Command(bin, mode, str).CombinedOutput()
	if err != nil {
		t.Fatalf("OCCT %s %s: %v (%s)", mode, str, err, raw)
	}
	var valid, solids, faces int
	var volume, area float64
	if _, err := fmt.Sscan(string(raw), &valid, &solids, &faces, &volume, &area); err != nil {
		t.Fatalf("could not read OCCT properties: %v (%s)", err, raw)
	}
	if valid != 1 || solids != wantSolids || faces != len(solid.Faces()) {
		t.Fatalf("OCCT %s %s topology=%q, want solids=%d faces=%d", mode, str, raw, wantSolids, len(solid.Faces()))
	}
	if solids == 0 {
		return
	}
	p, err := solid.MassProperties(context.Background(), IntegrationOptions{})
	if err != nil || math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
		t.Fatalf("OCCT %s %s properties=%q, native=%+v (%v)", mode, str, raw, p, err)
	}
}
