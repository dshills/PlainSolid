package heal

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"slices"
	"testing"
)

func TestSewOrientsNestedMaterialAndTracksInputFaces(t *testing.T) {
	ctx := context.Background()
	xyz := func(k float64) geom.Vec3 { return geom.Vec3{X: k, Y: k, Z: k} }
	// Put the cavity first, so its parent follows it in the reported order.
	var input []Face
	for _, span := range [][2]float64{{1, 7}, {2, 4}, {0, 8}, {10, 11}} {
		box, err := brep.Box(xyz(span[0]), xyz(span[1]))
		if err != nil {
			t.Fatal(err)
		}
		input = append(input, independent(box)...)
	}
	for _, i := range []int{6, 10, 15, 18} {
		reverseInput(&input[i])
	}
	options := collision.LocationOptions{}
	solid, report, err := SewContext(ctx, input, Options{OrientByNesting: &options})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Shells) != 4 || !slices.Equal(report.FlippedFaces, []int{0, 1, 2, 3, 4, 5, 6, 10, 15, 18}) {
		t.Fatalf("shells %+v; relative face flips %v", report.Shells, report.FlippedFaces)
	}
	parents, depths, signed := []int{2, 0, -1, -1}, []int{1, 2, 0, 0}, []int64{-216, 8, 512, 1}
	for i, shell := range report.Shells {
		wantFaces := []int{i * 6, i*6 + 1, i*6 + 2, i*6 + 3, i*6 + 4, i*6 + 5}
		if shell.Parent != parents[i] || shell.Depth != depths[i] || shell.SignedVolume.Cmp(big.NewRat(signed[i], 1)) != 0 ||
			shell.Reversed != (i != 2) || !slices.Equal(shell.Faces, wantFaces) {
			t.Fatalf("shell %d: %+v", i, shell)
		}
	}
	for f, face := range input {
		for i, original := range face.Outer {
			if report.Vertices[report.VertexMap[f].Outer[i]] != original {
				t.Fatalf("face %d vertex %d lost input ancestry", f, i)
			}
		}
	}
	near(t, solid.Tessellate().SignedVolume(), 305, 1e-12)
	locator, err := collision.NewPointLocator(ctx, solid, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		x float64
		w collision.Location
	}{{-1, collision.Outside}, {.5, collision.Inside}, {1, collision.OnBoundary}, {1.5, collision.Outside}, {3, collision.Inside}, {10.5, collision.Inside}} {
		got, err := locator.Locate(ctx, xyz(tc.x))
		if err != nil || got.Location != tc.w {
			t.Fatalf("point %g: %+v, %v; want %v", tc.x, got, err, tc.w)
		}
	}
	// Inspection preserves the caller's winding; a classifier remains strict.
	raw, _, err := Sew(input, Options{RepairOrientation: true})
	if err != nil {
		t.Fatal(err)
	}
	shells, err := collision.InspectShellNesting(ctx, raw, options)
	if err != nil || len(shells) != 4 || shells[0].SignedVolume.Cmp(big.NewRat(216, 1)) != 0 || shells[0].Parent != 2 {
		t.Fatalf("inspection: %+v, %v", shells, err)
	}
	if p, err := collision.NewPointLocator(ctx, raw, options); p != nil || !errors.Is(err, collision.ErrInvalid) {
		t.Fatalf("incorrect material winding admitted: %v, %v", p, err)
	}
}

func TestSewNestingRejectsIntersectionsContactsAndUnfinishedProofs(t *testing.T) {
	ctx := context.Background()
	base, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	options := collision.LocationOptions{}
	for _, tc := range []struct {
		name string
		x    float64
	}{{"crossing", 3}, {"touching with separate vertices", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			box, err := brep.Box(geom.Vec3{X: tc.x, Y: 1, Z: 1}, geom.Vec3{X: 5, Y: 2, Z: 2})
			if err != nil {
				t.Fatal(err)
			}
			input := append(independent(base), independent(box)...)
			if solid, _, err := Sew(input, Options{}); err != nil || solid.Topology().Shells != 2 {
				t.Fatalf("fixture must have closed incidence: %v", err)
			}
			solid, report, err := SewContext(ctx, input, Options{OrientByNesting: &options})
			if !errors.Is(err, ErrInvalid) || !errors.Is(err, collision.ErrInvalid) || len(solid.Faces()) != 0 || len(report.Shells) != 0 || len(report.VertexMap) != 12 {
				t.Fatalf("contact escaped proof: %v, %+v", err, report)
			}
		})
	}
	for _, opts := range []collision.LocationOptions{{MaxWork: 1}, {MaxTriangles: 1}, {MaxCollisionPairs: 1}} {
		solid, report, err := Sew(cubeFaces(), Options{OrientByNesting: &opts})
		if !errors.Is(err, ErrLimit) || len(solid.Faces()) != 0 || len(report.Shells) != 0 {
			t.Fatalf("budget %+v: %v, %+v", opts, err, report)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := SewContext(canceled, cubeFaces(), Options{OrientByNesting: &options}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := Sew(cubeFaces(), Options{OrientOutward: true, OrientByNesting: &options}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
