package step

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func verifyOCCTMappedChainSTEP(t *testing.T, raw []byte, reordered bool) MappedChainSTEPModel {
	t.Helper()
	ctx := context.Background()
	var o MappedChainShellSTEPOptions
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if m.OriginalSTEP() != string(raw) || m.LengthUnit != .001 || len(m.Planes) != 5 || len(m.Chain) != 3 ||
		len(m.Native.Solid.Vertices()) != 12 || len(m.Native.Solid.Edges()) != 17 || len(m.FaceIDs) != 7 {
		t.Fatal("qualified source ownership or native shell inventory")
	}
	if _, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o); err == nil {
		t.Fatal("ordinary canonical entry must retain its qualified input domain")
	}
	faces := m.Native.Solid.Faces()
	seen := make(map[int64]bool)
	if reordered {
		var bid int64
		for _, id := range f.Order {
			if _, ok := component(f.Entities[id], "MANIFOLD_SOLID_BREP"); ok {
				bid = id
			}
		}
		ordered, err := bodyFaceIDs(f, bid)
		if err != nil || slices.Equal(ordered, m.FaceIDs) {
			t.Fatal("live fixture must genuinely reorder the source faces", err)
		}
		for i := range 2 {
			rawFace, err := parseMappedChainStepFace(f, m.FaceIDs[i], true, 0)
			if err != nil || rawFace.uses[0].id == m.Chain[0].EdgeID {
				t.Fatal("live fixture must start both roof wires off the shared chain", i, err)
			}
		}
	}
	for i, face := range faces[:2] {
		r, err := simple(f, m.FaceIDs[i], "ADVANCED_FACE", 4)
		if err != nil {
			t.Fatal(err)
		}
		support, err := DecodeBSplineSurface(f, r.Args[2].Ref, 4)
		if err != nil || !geom.SameSplineSurface(support.Geometry, face.Surface.(*geom.BSplineSurface)) {
			t.Fatal("changed original spline chart", i, err)
		}
	}
	for i, cell := range m.Chain {
		if m.Native.ChainStations[i].Cmp(big.NewRat(int64(i), 3)) != 0 {
			t.Fatal("derived station order")
		}
		for k, u := range cell.Owners {
			if u.FaceID != m.FaceIDs[k] || u.TrimmedCurveID != 0 || seen[u.BasisID] {
				t.Fatal("lost independent roof pcurve ownership", i, k)
			}
			seen[u.BasisID] = true
			pc, err := simple(f, u.PCurveID, "PCURVE", 3)
			if err != nil {
				t.Fatal(err)
			}
			s, err := readMappedChainStepSpline(f, u.BasisID, 2, 3, 4, true)
			if err != nil {
				t.Fatal(err)
			}
			trim := faces[k].Outer[i].Curve.(trimmed.MappedSplineUV)
			source := trim.Source()
			if source.Range != (trimmed.UVInterval{Max: 1}) || trim.ParameterWeight().Cmp(big.NewRat(1, 1)) != 0 ||
				source.Curve.Degree() != s.degree || pc.Args[1].Ref == 0 {
				t.Fatal("changed separate source basis, span or speed", i, k)
			}
			for j, p := range source.Curve.Poles() {
				if mappedChainStepRat(p.X).Cmp(s.poles[j][0]) != 0 || mappedChainStepRat(p.Y).Cmp(s.poles[j][1]) != 0 {
					t.Fatal("changed source raw pcurve pole")
				}
			}
		}
	}
	var sourcedDecimal bool
	for _, plane := range m.Planes {
		if plane.OriginalOrigin == [3]string{"1", "218334351761/500000000000", "-46966991411/200000000000"} {
			sourcedDecimal = true
			if plane.UVShift != [2]string{"-46966991411/200000000000", "-218334351761/500000000000"} ||
				plane.Directions != [2][3]int{{0, 0, 1}, {0, -1, 0}} {
				t.Fatal("lost exact original analytic plane coordinates", plane)
			}
			owner := slices.Index(m.FaceIDs, plane.FaceID)
			model, err := faces[owner].Surface.Eval(.25, -.25)
			if err != nil || model != (geom.Vec3{X: 1, Y: .25, Z: .25}) {
				t.Fatal("incorrect exact shift on original wall", model, err)
			}
		}
	}
	if !sourcedDecimal {
		t.Fatal("expected OCCT's exact nonbinary planar origin")
	}
	var native bytes.Buffer
	if err := trimmed.WriteJSON(&native, m.Native.Solid); err != nil {
		t.Fatal(err)
	}
	solid, err := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	var sourceRecipe bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &sourceRecipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal("persist independent original source coedges", err)
	}
	reopened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &sourceRecipe, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rebuild, err := reopened.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var original, reconstructed bytes.Buffer
	if err := trimmed.WriteJSON(&original, m.Native.Solid); err != nil {
		t.Fatal(err)
	}
	if err := trimmed.WriteJSON(&reconstructed, rebuild.Solid); err != nil || !bytes.Equal(original.Bytes(), reconstructed.Bytes()) {
		t.Fatal("independent source recipe did not restore the complete native shell", err)
	}

	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 34 {
		t.Fatal("standalone native coedges after source transfer", err)
	}
	p, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	wantCenter := geom.Vec3{X: 10.0 / 19, Y: 15.0 / 38, Z: -15.0 / 38}
	if err != nil || math.Abs(p.Volume-57.0/64) > 1e-8 || math.Abs(p.Area-5.8254446210027595) > 1e-8 || p.Centroid.Sub(wantCenter).Length() > 1e-8 {
		t.Fatal("imported original analytical mass", p, err)
	}
	t.Logf("source entities=%d volume=%.12g area=%.15g centroid=%v", len(f.Entities), p.Volume, p.Area, p.Centroid)
	return m
}

func TestOCCTMappedChainRectangularRecordedSTEP(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/occt_mapped_chain_rectangular.step")
	if err != nil {
		t.Fatal(err)
	}
	model := verifyOCCTMappedChainSTEP(t, raw, true)
	ctx := context.Background()
	for _, change := range []struct {
		name string
		edit func(*File)
	}{
		{"different original plane origin", func(f *File) {
			for _, p := range model.Planes {
				if p.OriginalOrigin[0] != "1" {
					continue
				}
				plane := f.Entities[p.SupportID].Records[0]
				frame := f.Entities[plane.Args[1].Ref].Records[0]
				origin := f.Entities[frame.Args[1].Ref].Records[0]
				origin.Args[1].Items[1].Text = "0.436668703523"
				return
			}
		}},
		{"borrowed direct roof pcurve basis", func(f *File) {
			a := f.Entities[model.Chain[0].Owners[0].PCurveID].Records[0]
			b := f.Entities[model.Chain[0].Owners[1].PCurveID].Records[0]
			rep := f.Entities[a.Args[2].Ref].Records[0]
			rep.Args[1].Items[0] = f.Entities[b.Args[2].Ref].Records[0].Args[1].Items[0]
		}},
		{"different interior model spline pole", func(f *File) {
			r := f.Entities[model.Chain[1].ModelCurveID].Records[0]
			p := f.Entities[r.Args[2].Items[3].Ref].Records[0]
			p.Args[1].Items[2].Text = "0.03125"
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			change.edit(f)
			out, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, MappedChainShellSTEPOptions{})
			if err == nil || len(out.Native.Solid.Vertices()) != 0 {
				t.Fatal("changed original foreign source condition accepted")
			}
		})
	}
}

func TestOCCTMappedChainRectangularLiveAuthor(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_AUTHOR")
	if exe == "" {
		t.Skip("optional pinned OCCT source author")
	}
	for _, mode := range []string{"write", "write-reordered"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "occt-original.step")
			if out, err := exec.CommandContext(ctx, exe, mode, path).CombinedOutput(); err != nil {
				t.Fatalf("independent OCCT author: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			verifyOCCTMappedChainSTEP(t, raw, mode == "write-reordered")
		})
	}
}
