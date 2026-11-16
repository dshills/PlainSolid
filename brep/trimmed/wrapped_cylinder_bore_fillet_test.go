package trimmed

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

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func eccentricCylinderBoreFilletFixture(t *testing.T, radius float64) Solid {
	t.Helper()
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	outer, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{Z: 2}, radius, 8, o)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: 1.5, Y: 2, Z: 2}, 2, 8, o)
	if err != nil {
		t.Fatal(err)
	}
	source, _, err := NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestEccentricCircularBoreRimFillet(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	const radius = 6.25
	stock := eccentricCylinderBoreFilletFixture(t, radius)
	for _, c := range []struct {
		name         string
		lower, upper float64
		faces        int
	}{
		{"lower", .5, 0, 5},
		{"upper", 0, .75, 5},
		{"both", .5, .75, 6},
	} {
		t.Run(c.name, func(t *testing.T) {
			result, h, err := FilletWrappedCylinderBoreRims(ctx, stock, c.lower, c.upper, o)
			if err != nil {
				t.Fatal(err)
			}
			top := result.Topology()
			if len(top.Shells) != 1 || top.EulerCharacteristic != 0 || top.InnerWires != 2 || top.Faces != c.faces {
				t.Fatalf("annular topology %+v", top)
			}
			if result.faces[h.Outer].Surface != stock.faces[h.SourceOuter].Surface {
				t.Fatal("outer support changed")
			}
			for end, cap := range h.Caps {
				if result.faces[cap].Surface != stock.faces[h.SourceCaps[end]].Surface {
					t.Fatal("a source cap plane changed")
				}
			}
			const boreRadius, height = 2., 8.
			volume := math.Pi * (radius*radius - boreRadius*boreRadius) * height
			area := 2 * math.Pi * ((radius+boreRadius)*height + radius*radius - boreRadius*boreRadius)
			for _, r := range []float64{c.lower, c.upper} {
				R := boreRadius + r
				// Integrals of the inner quarter-circle radius
				// rho(z)=R-sqrt(r²-z²), with z measured from the shoulder.
				volume -= math.Pi * ((R*R-boreRadius*boreRadius)*r - R*r*r*math.Pi/2 + 2*r*r*r/3)
				area += math.Pi*(boreRadius*boreRadius-R*R) + 2*math.Pi*r*(R*math.Pi/2-r-boreRadius)
			}
			mass, err := result.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(mass.Volume-volume) > 1e-7 || math.Abs(mass.Area-area) > 1e-7 {
				t.Fatalf("integrated %+.12g volume and %+.12g area, expected %+.12g and %+.12g: %v", mass.Volume, mass.Area, volume, area, err)
			}
			checkCircularBoreFilletOracle(t, ctx, c.name, c.lower, c.upper, c.faces, mass.Volume, mass.Area)
			var native bytes.Buffer
			if err := WriteJSON(&native, result); err != nil {
				t.Fatal(err)
			}
			loaded, err := ReadJSONContext(ctx, bytes.NewReader(native.Bytes()), JSONLimits{})
			if err != nil {
				t.Fatal(err)
			}
			var replay bytes.Buffer
			if err := WriteJSON(&replay, loaded); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
				t.Fatalf("native replay changed: %v", err)
			}
			proof, err := loaded.ValidateWrappedCylinderBoreRimFillet(ctx, o)
			if err != nil || !proof.Complete || proof.Blends != h.Blends || proof.Caps != h.Caps ||
				proof.Fillets[0].Cmp(ratInset(c.lower)) != 0 || proof.Fillets[1].Cmp(ratInset(c.upper)) != 0 {
				t.Fatalf("exact reproof of original blend owners: %+v, %v", proof, err)
			}
			for end, r := range []float64{c.lower, c.upper} {
				if r > 0 {
					checkCircularBoreFilletTangency(t, ctx, loaded, proof, end)
				}
			}
			m, err := loaded.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 20000})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if len(m.Vertices) < len(loaded.vertices) || !slices.Equal(m.Vertices[:len(loaded.vertices)], loaded.vertices) {
				t.Fatal("display discarded native topology vertices")
			}
			if c.name == "both" {
				checkCircularBoreFilletEditedChart(t, ctx, loaded, proof.Blends[1])
				contacts, err := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
				if err != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
					t.Fatalf("display self-intersection check: %+v, %v", contacts, err)
				}
				limited, err := loaded.Tessellate(ctx, parametric.TessellationOptions{
					Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: len(m.Triangles) - 1,
				})
				if !errors.Is(err, parametric.ErrBudget) || len(limited.Triangles) != 0 {
					t.Fatalf("triangle limit admitted partial geometry: %v", err)
				}
			}
			t.Logf("faces %d / volume %.16g / area %.16g / closed display %d triangles", top.Faces, mass.Volume, mass.Area, len(m.Triangles))
		})
	}
	t.Run("limits", func(t *testing.T) {
		for _, r := range []float64{radius - 4.5, radius - 4.375} {
			if failed, history, err := FilletWrappedCylinderBoreRims(ctx, stock, r, 0, o); !errors.Is(err, geom.ErrDomain) ||
				len(failed.faces) != 0 || history != (WrappedCylinderBoreFilletHistory{}) {
				t.Fatalf("outer cylinder tangency or crossing at radius %g: %v", r, err)
			}
		}
		thick := eccentricCylinderBoreFilletFixture(t, 16)
		if failed, _, err := FilletWrappedCylinderBoreRims(ctx, thick, 4, 4, o); !errors.Is(err, geom.ErrDomain) || len(failed.faces) != 0 {
			t.Fatalf("touching fillet bands consumed the bore wall: %v", err)
		}
		if failed, _, err := FilletWrappedCylinderBoreRims(ctx, stock, .5, 0, geom.SurfacePatchOptions{MaxOperations: 1}); err == nil || len(failed.faces) != 0 {
			t.Fatalf("operation limit admitted a partial shape: %v", err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if failed, _, err := FilletWrappedCylinderBoreRims(canceled, stock, .5, 0, o); !errors.Is(err, context.Canceled) || len(failed.faces) != 0 {
			t.Fatalf("canceled operation admitted a partial shape: %v", err)
		}
	})
}

func checkCircularBoreFilletEditedChart(t *testing.T, ctx context.Context, original Solid, fillet int) {
	t.Helper()
	w := original.faces[fillet].Surface.(*geom.WrappedSurfacePatch)
	basis, err := w.FundamentalPatch().ToBSplineSurface()
	if err != nil {
		t.Fatal(err)
	}
	data := basis.Data()
	// This changes the meridian between the original two rim sections only.
	// The edited patch keeps both exact cap and shoulder curves, plus an
	// exactly periodic seam, so those facts cannot certify the whole blend.
	for u := range data.CountU {
		data.Poles[u*data.CountV+1].Z += .125
	}
	spline, err := geom.NewBSplineSurfaceWithExactWeights(data.DegreeU, data.DegreeV, data.CountU, data.CountV,
		data.Poles, data.KnotsU, data.KnotsV, data.Weights)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := geom.NewSurfacePatch(spline, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	edited, _, err := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	tampered := original
	tampered.faces, tampered.edges = original.Faces(), original.Edges()
	tampered.faces[fillet].Surface = edited
	d := edited.Domain()
	for _, use := range tampered.faces[fillet].Outer {
		if use.UVStart.X == d.UMin && use.UVEnd.X == d.UMin {
			tampered.edges[use.Edge].Curve = surfaceTrimCurve{edited, geom.Vec2{Y: d.VMin}, geom.Vec2{Y: d.VMax}, nil}
		}
	}
	if proof, err := tampered.ValidateWrappedCylinderBoreRimFillet(ctx, geom.SurfacePatchOptions{}); !errors.Is(err, geom.ErrConvergence) || proof.Complete {
		t.Fatalf("edited toroidal meridian passed exact proof: %+v, %v", proof, err)
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, tampered); err != nil {
		t.Fatal(err)
	}
	if accepted, err := ReadJSONContext(ctx, &native, JSONLimits{}); !errors.Is(err, geom.ErrConvergence) || len(accepted.faces) != 0 {
		t.Fatalf("edited toroidal meridian passed native replay: %v", err)
	}
}

func checkCircularBoreFilletTangency(t *testing.T, ctx context.Context, solid Solid, p WrappedCylinderBoreFilletProof, end int) {
	t.Helper()
	blend := solid.faces[p.Blends[end]].Surface.(*geom.WrappedSurfacePatch)
	shoulder := [2]*big.Rat{p.Hole.LowerZ, p.Hole.UpperZ}[end]
	major := new(big.Rat).Add(p.Hole.Radius, p.Fillets[end])
	for _, v := range []*big.Rat{big.NewRat(2, 1), big.NewRat(7-int64(end)*3, 3), big.NewRat(3-int64(end)*2, 1)} {
		jet, err := blend.EvaluateRational(ctx, big.NewRat(0, 1), v, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		x := new(big.Rat).Sub(jet.Point[0], p.Hole.CenterX)
		dX := new(big.Rat).Sub(x, major)
		dZ := new(big.Rat).Sub(jet.Point[2], shoulder)
		d2 := new(big.Rat).Add(new(big.Rat).Mul(dX, dX), new(big.Rat).Mul(dZ, dZ))
		if d2.Cmp(new(big.Rat).Mul(p.Fillets[end], p.Fillets[end])) != 0 || jet.Point[1].Cmp(p.Hole.CenterY) != 0 {
			t.Fatal("exact meridian leaves its rolling circle")
		}
		if v.Cmp(big.NewRat(2, 1)) == 0 {
			if dZ.Sign() != 0 || jet.Dv[0].Sign() != 0 || jet.Dv[2].Sign() >= 0 {
				t.Fatal("blend is not tangent to the original cylindrical bore")
			}
		} else if v.IsInt() && (dX.Sign() != 0 || jet.Dv[2].Sign() != 0 || jet.Dv[0].Sign() == 0) {
			t.Fatal("blend is not tangent to the original cap plane")
		}
	}
}

func checkCircularBoreFilletOracle(t *testing.T, ctx context.Context, mode string, lower, upper float64, faces int, volume, area float64) {
	t.Helper()
	bin := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_BORE_RIM_FILLET")
	if bin == "" {
		return
	}
	data, err := exec.CommandContext(ctx, bin, mode).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT rim fillet: %s, %v", data, err)
	}
	var result struct {
		Solids, Faces int
		Volume, Area  float64
		Rims          [][4]float64
		Toroids       [][5]float64
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Solids != 1 || result.Faces != faces ||
		math.Abs(result.Volume-volume) > 1e-7 || math.Abs(result.Area-area) > 1e-7 {
		t.Fatalf("pinned OCCT mass/topology %s: %v", data, err)
	}
	wantRims := [][4]float64{{0, 0, 2, 6.25}, {0, 0, 10, 6.25}, {1.5, 2, 2, 2 + lower}, {1.5, 2, 10, 2 + upper}}
	var wantToroids [][5]float64
	for end, r := range []float64{lower, upper} {
		if r == 0 {
			continue
		}
		z := [2]float64{2 + r, 10 - r}[end]
		wantRims = append(wantRims, [4]float64{1.5, 2, z, 2})
		wantToroids = append(wantToroids, [5]float64{1.5, 2, z, 2 + r, r})
	}
	for _, want := range wantRims {
		if !slices.Contains(result.Rims, want) {
			t.Fatalf("missing original circular cap or shoulder %v: %s", want, data)
		}
	}
	for _, want := range wantToroids {
		if !slices.Contains(result.Toroids, want) || len(result.Toroids) != len(wantToroids) {
			t.Fatalf("incorrect original rolling circle %v: %s", want, data)
		}
	}
	t.Logf("pinned OCCT matched %d cap and shoulder circles, %d rolling supports, volume and area", len(wantRims), len(wantToroids))
}
