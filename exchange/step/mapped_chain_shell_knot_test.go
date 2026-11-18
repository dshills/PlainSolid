package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
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

func verifyCreasedMappedSTEP(t *testing.T, model MappedChainSTEPModel) parametric.Properties {
	t.Helper()
	ctx := context.Background()
	solid := model.Native.Solid
	if len(solid.Vertices()) != 14 || len(solid.Edges()) != 19 || len(solid.Faces()) != 7 || len(model.Chain) != 3 {
		t.Fatal("original support knot station vertices must survive across both closures")
	}
	axis := [2]int{}
	for i, f := range solid.Faces()[:2] {
		source := f.Surface.(*geom.BSplineSurface).Data()
		u := slices.Equal(source.KnotsU, []float64{0, 0, .5, 1, 1})
		v := slices.Equal(source.KnotsV, []float64{0, 0, .5, 1, 1})
		if u == v {
			t.Fatal("lost actual original support knot", i, source.KnotsU, source.KnotsV)
		}
		if v {
			axis[i] = 1
		}
	}
	if axis[0] == axis[1] {
		t.Fatal("the original charts carry the shared crease on different UV axes")
	}
	report, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !report.Complete || len(report.Uses) != 38 {
		t.Fatal("complete original coedge ownership after multi-span source import", err)
	}
	mass, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	center := geom.Vec3{X: 13.0 / 24, Y: 63.0 / 160, Z: -341.0 / 960}
	if err != nil || math.Abs(mass.Volume-15.0/16) > 1e-8 || math.Abs(mass.Area-6.2184181384796142) > 1e-8 ||
		mass.Centroid.Sub(center).Length() > 1e-8 {
		t.Fatal("original creased-roof material", mass, err)
	}
	return mass
}

func creasedCanonicalMappedSTEP(t *testing.T) []byte {
	t.Helper()
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/mapped_chain_creased_source.json")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &data, shell, "Creased mapped chain", .001, Limits{}, MappedChainShellSTEPOptions{}); err == nil || data.Len() != 0 {
		t.Fatal("multi-span STEP writer needs the explicit source capability")
	}
	if err := WriteMappedChainRectangularShell(ctx, &data, shell, "Creased mapped chain", .001, Limits{}, MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true,
	}); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestMappedChainRectangularSTEPOriginalSupportKnot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	file := creasedCanonicalMappedSTEP(t)
	if _, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{}); err == nil {
		t.Fatal("multi-span STEP reader needs the explicit source capability")
	}
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true}
	model, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	mass := verifyCreasedMappedSTEP(t, model)
	second := new(bytes.Buffer)
	if err := WriteMappedChainRectangularShell(ctx, second, model.Shell, model.Name, model.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(file, second.Bytes()) {
		t.Fatal("original authored knot data must round-trip canonically", err)
	}
	p, err := Parse(bytes.NewReader(file), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	face := p.Entities[model.FaceIDs[0]].Records[0]
	support := p.Entities[face.Args[2].Ref].Records[0]
	support.Args[10].Items[1].Text = "0.625"
	bad, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(p)), Limits{}, o)
	if err == nil || len(bad.Native.Solid.Faces()) != 0 {
		t.Fatal("changed interior support knot escaped exact original chart proof")
	}
	t.Logf("crease STEP bytes=%d volume=%.12g area=%.15g center=%v", len(file), mass.Volume, mass.Area, mass.Centroid)
}

func TestMappedChainRectangularSTEPOriginalSupportKnotOCCT(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	file := creasedCanonicalMappedSTEP(t)
	path := filepath.Join(t.TempDir(), "creased-canonical.step")
	if err := os.WriteFile(path, file, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("OCCT multi-span STEP transfer %v\n%s", err, data)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	if props.Faces != 7 || props.Solids != 1 || math.Abs(props.Volume-15.0/16) > 1e-8 ||
		math.Abs(props.Area-6.2184181384796142) > 1e-8 {
		t.Fatal("independent OCCT sees the wrong creased material", props)
	}
	t.Logf("OCCT read 1 solid, 7 faces, volume %.15g, area %.15g", props.Volume, props.Area)
}

func verifyForeignCreasedMappedSTEP(t *testing.T, file []byte) parametric.Properties {
	t.Helper()
	ctx := context.Background()
	if _, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{}); err == nil {
		t.Fatal("multi-span foreign transfer needs the explicit source capability")
	}
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, o)
	if err != nil || m.OriginalSTEP() != string(file) || len(m.Planes) != 5 {
		t.Fatal("foreign source maps, original data or proof", err)
	}
	p := verifyCreasedMappedSTEP(t, m)
	var recipe bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal("persist the original independent roofs and support knots", err)
	}
	loaded, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := loaded.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for owner, f := range rebuilt.Solid.Faces()[:2] {
		original := m.Native.Solid.Faces()[owner].Surface.(*geom.BSplineSurface)
		if !geom.SameSplineSurface(f.Surface.(*geom.BSplineSurface), original) {
			t.Fatal("changed original foreign roof chart on native reopen")
		}
	}
	t.Logf("OCCT-authored knot transfer volume=%.12g area=%.15g center=%v", p.Volume, p.Area, p.Centroid)
	return p
}

func TestOCCTMappedChainRectangularOriginalSupportKnotRecorded(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("testdata/occt_mapped_chain_creased.step")
	if err != nil {
		t.Fatal(err)
	}
	verifyForeignCreasedMappedSTEP(t, b)
}

func TestOCCTMappedChainRectangularOriginalSupportKnotLive(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_AUTHOR")
	if exe == "" {
		t.Skip("optional pinned OCCT source author")
	}
	for _, mode := range []string{"write-knot", "write-knot-default-planes"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "creased-source.step")
			if out, err := exec.CommandContext(ctx, exe, mode, path).CombinedOutput(); err != nil {
				t.Fatalf("independent author %v\n%s", err, out)
			}
			file, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "write-knot" {
				verifyForeignCreasedMappedSTEP(t, file)
				return
			}
			m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(file), Limits{}, MappedChainShellSTEPOptions{
				AllowMultiSpanRoofs: true,
			})
			if err == nil || len(m.Native.Solid.Faces()) != 0 {
				t.Fatal("arithmetic mismatch in auto-centered source planar lifts must fail exact transfer")
			}
			t.Logf("rejected independently authored default-plane control: %v", err)
		})
	}
}
