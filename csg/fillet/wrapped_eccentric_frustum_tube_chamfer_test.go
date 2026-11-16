package fillet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestEccentricAnnularFrustumFourRimChamfers(t *testing.T) {
	checkEccentricAnnularFrustumFourRimChamfers(t, geom.Vec2{X: 2.5}, nil)
}

func TestDiagonalEccentricAnnularFrustumFourRimChamfers(t *testing.T) {
	checkEccentricAnnularFrustumFourRimChamfers(t, geom.Vec2{X: 1.5, Y: 2}, []string{"diagonal"})
}

func checkEccentricAnnularFrustumFourRimChamfers(t *testing.T, center geom.Vec2, oracleArgs []string) {
	t.Helper()
	ctx := context.Background()
	opts := geom.SurfacePatchOptions{}
	source := eccentricChamferFixture(t, ctx, center)
	before, err := source.ValidateCanonicalWrappedFrustumBore(ctx, opts)
	if err != nil || !before.Complete || before.Kind != "tube" {
		t.Fatalf("original four-face frustum tube: %v", err)
	}
	sel := CircularTubeChamfers{
		Outer: [2]CircularRimChamfer{{.25, 2}, {.5, 1}},
		Inner: [2]CircularRimChamfer{{.1875, 1}, {.375, 2}},
	}
	if legacy, err := ChamferWrappedFrustumTubeRims(ctx, source, sel, opts); !errors.Is(err, geom.ErrConvergence) || len(legacy.SourceFaces) != 0 {
		t.Fatalf("coaxial operation changed its eccentric rejection: %v", err)
	}
	result, err := ChamferWrappedEccentricFrustumTubeRims(ctx, source, sel, opts)
	if err != nil {
		t.Fatal(err)
	}
	top := result.Solid.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 0 || top.InnerWires != 2 || top.Faces != 12 {
		t.Fatalf("invalid annular topology: %+v", top)
	}
	proof, err := result.Solid.ValidateWrappedEccentricCircularStationBore(ctx, opts)
	if err != nil || !proof.Complete {
		t.Fatalf("independently restored eccentric station bore: %v", err)
	}
	checkEccentricRimChamferStations(t, proof, center)
	if proof.Caps != result.Caps || !slices.Equal(proof.OuterWalls, result.Walls[0]) || !slices.Equal(proof.InnerWalls, result.Walls[1]) {
		t.Fatalf("original result face IDs changed during reproof: %+v", proof)
	}
	for i, cap := range result.Caps {
		if !slices.Equal(result.SourceFaces[cap], []int{before.OuterFaces[i+1]}) {
			t.Fatalf("cap %d lost its original owner", i)
		}
	}
	counts := [2][2]int{{2, 1}, {1, 2}}
	for side, wall := range [2]int{before.OuterFaces[0], before.InnerFaces[0]} {
		for end, bevels := range result.Bevels[side] {
			if len(bevels) != counts[side][end] {
				t.Fatalf("wall %d end %d lost a bevel band: %v", side, end, bevels)
			}
			for _, bevel := range bevels {
				if !slices.Equal(result.SourceFaces[bevel], []int{wall, before.OuterFaces[end+1]}) {
					t.Fatalf("bevel %d lost its wall/cap owners", bevel)
				}
			}
		}
		for _, id := range result.Walls[side] {
			if !slices.Contains(result.Bevels[side][0], id) && !slices.Contains(result.Bevels[side][1], id) &&
				!slices.Equal(result.SourceFaces[id], []int{wall}) {
				t.Fatalf("unchanged original wall band %d lost its owner", id)
			}
		}
	}
	edgeAgreement, err := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !edgeAgreement.Complete {
		t.Fatalf("exact edge agreement: %v", err)
	}
	for _, use := range edgeAgreement.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("a cap or wall coedge fails exact equality to the model curve")
		}
	}
	mass, err := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	// These are analytic integrals of the eight expected cap/shoulder
	// circles, independent of the result's split wall partitions.
	volume, area := eccentricRimChamferMeasures()
	if math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
		t.Fatalf("native volume %.16g area %.16g; analytical %.16g and %.16g", mass.Volume, mass.Area, volume, area)
	}
	t.Logf("native four-rim chamfer: %d faces, volume %.16g, area %.16g", top.Faces, mass.Volume, mass.Area)
	if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_RIM_CHAMFERS"); oracle != "" {
		checkOCCTEccentricRimChamfers(t, ctx, oracle, oracleArgs, center, mass.Volume, mass.Area)
	}
	var data bytes.Buffer
	if err := trimmed.WriteJSON(&data, result.Solid); err != nil {
		t.Fatal(err)
	}
	loaded, err := trimmed.ReadJSONContext(ctx, &data, trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := loaded.ValidateWrappedEccentricCircularStationBore(ctx, opts)
	if err != nil || !reloaded.Complete {
		t.Fatalf("station proof after native replay: %v", err)
	}
	checkEccentricRimChamferStations(t, reloaded, center)
	if other, err := loaded.ValidateWrappedCircularStationBore(ctx, opts); err == nil || other.Complete {
		t.Fatal("the coaxial validator must not admit the eccentric bore")
	}
	if len(oracleArgs) != 0 {
		checkDiagonalEccentricChamferDisplay(t, ctx, source)
		checkDiagonalEccentricChamferDisplay(t, ctx, loaded)
	}
}

func checkDiagonalEccentricChamferDisplay(t *testing.T, ctx context.Context, solid trimmed.Solid) {
	t.Helper()
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 20000}
	m, err := solid.Tessellate(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	original := solid.Vertices()
	if len(m.Vertices) < len(original) || !slices.Equal(original, m.Vertices[:len(original)]) {
		t.Fatal("display changed the original topology vertices")
	}
	report, err := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if err != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatalf("display self-intersection proof: %+v %v", report, err)
	}
	t.Logf("closed diagonal %d-face body: %d triangles, zero self-intersections", len(solid.Faces()), len(m.Triangles))
	o.MaxTriangles = len(m.Triangles) - 1
	if out, err := solid.Tessellate(ctx, o); !errors.Is(err, parametric.ErrBudget) || len(out.Triangles) != 0 {
		t.Fatalf("display triangle limit yielded partial mesh: %v", err)
	}
}

func TestEccentricAnnularFrustumChamferRejectsWallContact(t *testing.T) {
	ctx := context.Background()
	source := eccentricChamferFixture(t, ctx, geom.Vec2{X: 2.5})
	selection := CircularTubeChamfers{Outer: [2]CircularRimChamfer{{.25, 2}, {}}, Inner: [2]CircularRimChamfer{{.1875, 1}, {}}}
	for _, radial := range []float64{.3125, .375} {
		selection.Outer[0].Radial = radial
		result, err := ChamferWrappedEccentricFrustumTubeRims(ctx, source, selection, geom.SurfacePatchOptions{})
		if !errors.Is(err, geom.ErrDomain) || len(result.SourceFaces) != 0 || len(result.Solid.Faces()) != 0 {
			t.Fatalf("radial setback %g admits internally tangent or overlapping eccentric walls: %v", radial, err)
		}
	}
	selection.Outer[0].Radial = .25
	stop, cancel := context.WithCancel(ctx)
	cancel()
	if result, err := ChamferWrappedEccentricFrustumTubeRims(stop, source, selection, geom.SurfacePatchOptions{}); !errors.Is(err, context.Canceled) || len(result.SourceFaces) != 0 {
		t.Fatalf("cancelled operation returned partial geometry: %v", err)
	}
	if result, err := ChamferWrappedEccentricFrustumTubeRims(ctx, source, selection, geom.SurfacePatchOptions{MaxOperations: 1}); err == nil || len(result.SourceFaces) != 0 {
		t.Fatal("exhausted source reproof returned partial geometry")
	}
}

func eccentricChamferFixture(t *testing.T, ctx context.Context, center geom.Vec2) trimmed.Solid {
	t.Helper()
	opts := geom.SurfacePatchOptions{}
	wall := func(center geom.Vec3, lower, upper float64) *geom.WrappedSurfacePatch {
		t.Helper()
		support, err := geom.NewCanonicalWrappedCylinder(ctx, center, lower, 8, opts)
		if err != nil {
			t.Fatal(err)
		}
		support, _, err = geom.DraftWrappedFrustum(ctx, support, 0, upper-lower, opts)
		if err != nil {
			t.Fatal(err)
		}
		return support
	}
	outer := wall(geom.Vec3{Z: 2}, 5, 7)
	inner := wall(geom.Vec3{X: center.X, Y: center.Y, Z: 2}, 2, 3)
	source, _, err := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func checkEccentricRimChamferStations(t *testing.T, proof trimmed.WrappedStationBoreProof, center geom.Vec2) {
	t.Helper()
	heights := [...]float64{2, 3, 4, 8, 9, 10}
	radii := [2][6]float64{{4.75, 5.125, 5.5, 6.5, 6.75, 6.5}, {2.1875, 2.125, 2.25, 2.75, 3.0625, 3.375}}
	centers := [2][2]float64{{0, 0}, {center.X, center.Y}}
	for side, p := range [2]trimmed.WrappedStationLoftProof{proof.Outer, proof.Inner} {
		if !p.Complete || len(p.Heights) != len(heights) || len(p.Radii) != len(heights) {
			t.Fatalf("wall %d lost an original or split station", side)
		}
		for axis, c := range centers[side] {
			if p.Center[axis].Cmp(new(big.Rat).SetFloat64(c)) != 0 {
				t.Fatalf("wall %d changed its original axis", side)
			}
		}
		for i, z := range heights {
			if p.Heights[i].Cmp(new(big.Rat).SetFloat64(z)) != 0 || p.Radii[i].Cmp(new(big.Rat).SetFloat64(radii[side][i])) != 0 {
				t.Fatalf("wall %d station %d changed exact height or radius", side, i)
			}
		}
	}
}

func eccentricRimChamferMeasures() (volume, area float64) {
	z := [2][4]float64{{2, 4, 9, 10}, {2, 3, 8, 10}}
	r := [2][4]float64{{4.75, 5.5, 6.75, 6.5}, {2.1875, 2.125, 2.75, 3.375}}
	for side, sign := range [2]float64{1, -1} {
		for i := range 3 {
			h, a, b := z[side][i+1]-z[side][i], r[side][i], r[side][i+1]
			volume += sign * math.Pi * h * (a*a + a*b + b*b) / 3
			area += math.Pi * (a + b) * math.Hypot(h, b-a)
		}
		area += sign * math.Pi * (r[side][0]*r[side][0] + r[side][3]*r[side][3])
	}
	return volume, area
}

func checkOCCTEccentricRimChamfers(t *testing.T, ctx context.Context, oracle string, args []string, axis geom.Vec2, volume, area float64) {
	t.Helper()
	raw, err := exec.CommandContext(ctx, oracle, args...).CombinedOutput()
	var result struct {
		Solids, Faces int
		Volume, Area  float64
		Rims          [][4]float64
	}
	if err != nil || json.Unmarshal(raw, &result) != nil {
		t.Fatalf("independent OCCT BRepFilletAPI_MakeChamfer: %v\n%s", err, raw)
	}
	if result.Solids != 1 || result.Faces != 8 || math.Abs(result.Volume-volume) > 1e-7 || math.Abs(result.Area-area) > 1e-7 {
		t.Fatalf("OCCT solid: %+v; native volume %.16g area %.16g", result, volume, area)
	}
	expected := [][4]float64{
		{0, 0, 2, 4.75}, {0, 0, 4, 5.5}, {0, 0, 9, 6.75}, {0, 0, 10, 6.5},
		{axis.X, axis.Y, 2, 2.1875}, {axis.X, axis.Y, 3, 2.125}, {axis.X, axis.Y, 8, 2.75}, {axis.X, axis.Y, 10, 3.375},
	}
	if len(result.Rims) != len(expected) {
		t.Fatalf("OCCT changed one of the eight expected cap/shoulder rims: %v", result.Rims)
	}
	for _, rim := range expected {
		if !slices.ContainsFunc(result.Rims, func(actual [4]float64) bool {
			for i := range actual {
				if math.Abs(actual[i]-rim[i]) > 1e-9 {
					return false
				}
			}
			return true
		}) {
			t.Fatalf("OCCT did not retain the authored eccentric rim %v", rim)
		}
	}
	t.Logf("OCCT four-rim chamfer: one valid solid, eight expected cap/shoulder rims, volume %.16g, area %.16g", result.Volume, result.Area)
}
