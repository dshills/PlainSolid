package parametric

import (
	"context"
	"errors"
	"math"
	"math/big"
	"os"
	"testing"

	"plainsolid/geom"
)

func TestSeparatedSphereOffsets(t *testing.T) {
	ctx := context.Background()
	centers := [2]geom.Vec3{{}, {X: 4}}
	source, err := NewSpherePairSet(ctx, Ball{centers[0], 1}, Ball{centers[1], 2}, Union, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	const first, second = 1.75, 2.75
	const capFirst, capSecond = 5. / 16, 3. / 16
	lens := math.Pi * (capFirst*capFirst*(first-capFirst/3) + capSecond*capSecond*(second-capSecond/3))
	for _, tc := range []struct {
		distance     float64
		volume, area float64
		owners       [2]int
		transverse   bool
	}{
		{.25, 4 * math.Pi / 3 * (1.25*1.25*1.25 + 2.25*2.25*2.25), 4 * math.Pi * (1.25*1.25 + 2.25*2.25), [2]int{0, 1}, false},
		{.75, 4*math.Pi/3*(first*first*first+second*second*second) - lens, 4*math.Pi*(first*first+second*second) - 2*math.Pi*(capFirst*first+capSecond*second), [2]int{0, 1}, true},
		{-1, 4 * math.Pi / 3, 4 * math.Pi, [2]int{-1, 0}, false},
		{-2, 0, 0, [2]int{-1, -1}, false},
	} {
		solid, history, err := source.OffsetSeparatedSpheres(ctx, tc.distance, 1e-9)
		if err != nil || history.SourceFaces != [2]int{0, 1} || history.ResultFaces != tc.owners {
			t.Fatalf("offset %v owners %+v, error %v", tc.distance, history, err)
		}
		faces := solid.Faces()
		for i, id := range history.ResultFaces {
			radius := float64(i+1) + tc.distance
			if history.Radii[i].Cmp(new(big.Rat).SetFloat64(radius)) != 0 {
				t.Fatalf("offset %v original radius %d = %v, want %v", tc.distance, i, history.Radii[i], radius)
			}
			if id < 0 {
				continue
			}
			face := faces[id]
			support, ok := face.Surface.(geom.SphereSurface)
			if !ok || face.Reversed || (face.V.Min != -math.Pi/2 || face.V.Max != math.Pi/2) != tc.transverse {
				t.Fatalf("offset %v face %d has invalid topology", tc.distance, id)
			}
			frame, r := support.Parameters()
			if frame.Origin() != centers[i] || r != radius {
				t.Fatalf("offset %v face %d moved its original center or radius", tc.distance, id)
			}
		}
		if tc.volume == 0 {
			if len(faces) != 0 {
				t.Fatal("extinct union retained faces")
			}
			continue
		}
		p, err := solid.MassProperties(ctx, IntegrationOptions{})
		if err != nil || math.Abs(p.Volume-tc.volume) > 1e-7 || math.Abs(p.Area-tc.area) > 1e-7 {
			t.Fatalf("offset %v mass=%+v, want V=%v A=%v (%v)", tc.distance, p, tc.volume, tc.area, err)
		}
	}
	for _, tc := range []struct {
		distance float64
		want     error
	}{{.5, geom.ErrDomain}, {.1, geom.ErrUnrepresentable}} {
		solid, history, err := source.OffsetSeparatedSpheres(ctx, tc.distance, 1e-9)
		if !errors.Is(err, tc.want) || len(solid.Faces()) != 0 || history.Radii[0] != nil || history.ResultFaces != [2]int{-1, -1} {
			t.Fatalf("offset %v reported partial geometry %+v, %v", tc.distance, history, err)
		}
	}
	if _, _, err := sphericalOffsetFixture(t).OffsetSeparatedSpheres(ctx, .25, 1e-9); !errors.Is(err, geom.ErrDomain) {
		t.Fatalf("inward cavity face was accepted as a separated sphere: %v", err)
	}
}

func TestSeparatedSphereOffsetsOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET to the optional OCCT comparator")
	}
	ctx := context.Background()
	source, err := NewSpherePairSet(ctx, Ball{Radius: 1}, Ball{Center: geom.Vec3{X: 4}, Radius: 2}, Union, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		distance float64
		solids   int
	}{{.25, 2}, {.75, 1}, {-1, 1}, {-2, 0}} {
		solid, _, err := source.OffsetSeparatedSpheres(ctx, row.distance, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		checkSphereOffsetAgainstOCCT(t, bin, "separated-supports", row.distance, row.solids, solid)
	}
}
