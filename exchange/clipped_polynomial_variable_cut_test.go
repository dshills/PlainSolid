package exchange_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestVariableTransverseClippedCutExchange(t *testing.T) {
	ctx := context.Background()
	for _, count := range []int{1, 4, 5} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			fixture := fmt.Sprintf("testdata/native_clipped_polynomial_variable_cut_%d.json", count)
			source, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			cut, err := trimmed.ReadClippedPolynomialTransverseCutJSON(ctx, bytes.NewReader(source), trimmed.JSONLimits{}, trimmed.PolynomialNodeCutOptions{})
			if err != nil || len(cut.Contacts()) != count {
				t.Fatal("exact native source contact count", count, err)
			}
			const name, unit = "Variable transverse Cut", .001
			stepOptions := step.ClippedPolynomialCutSTEPOptions{PoleTolerance: 1e-14}
			igesOptions := iges.ClippedPolynomialCutIGESOptions{PoleTolerance: 1e-14}
			var stepData, igesData bytes.Buffer
			stepErr := step.WriteClippedPolynomialTransverseCut(ctx, &stepData, cut, name, unit, step.Limits{}, stepOptions)
			igesErr := iges.WriteClippedPolynomialTransverseCut(ctx, &igesData, cut, unit, iges.Limits{}, igesOptions)
			if count > 3 {
				if !errors.Is(stepErr, geom.ErrDomain) || !errors.Is(igesErr, geom.ErrDomain) || stepData.Len() != 0 || igesData.Len() != 0 {
					t.Fatal("unqualified thin-shell transfer must refuse before publishing records", stepErr, igesErr)
				}
				if err := step.VerifyClippedPolynomialTransverseCutSTEP(ctx, &stepData, cut, name, unit, step.Limits{}, stepOptions); !errors.Is(err, geom.ErrDomain) {
					t.Fatal("STEP verifier must share the transfer scope", err)
				}
				if err := iges.VerifyClippedPolynomialTransverseCutIGES(ctx, &igesData, cut, unit, iges.Limits{}, igesOptions); !errors.Is(err, geom.ErrDomain) {
					t.Fatal("IGES verifier must share the transfer scope", err)
				}
				return
			}
			if stepErr != nil || igesErr != nil {
				t.Fatal("one exact crossing must transfer in both formats", stepErr, igesErr)
			}
			if err := step.VerifyClippedPolynomialTransverseCutSTEP(ctx, bytes.NewReader(stepData.Bytes()), cut, name, unit, step.Limits{}, stepOptions); err != nil {
				t.Fatal(err)
			}
			if err := iges.VerifyClippedPolynomialTransverseCutIGES(ctx, bytes.NewReader(igesData.Bytes()), cut, unit, iges.Limits{}, igesOptions); err != nil {
				t.Fatal(err)
			}
			checkVariableCutSTEPContact(t, stepData.Bytes(), cut.Contacts()[0])
			checkVariableCutComponents(t, ctx, "step", stepData.Bytes(), os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), cut)
			checkVariableCutComponents(t, ctx, "igs", igesData.Bytes(), os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), cut)
		})
	}
}

func checkVariableCutSTEPContact(t *testing.T, raw []byte, contact trimmed.PolynomialMixedCutContact) {
	t.Helper()
	f, err := step.Parse(bytes.NewReader(raw), step.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	coords := map[int64]bool{}
	vertexCount := 0
	for _, e := range f.Entities {
		for _, r := range e.Records {
			if r.Type != "VERTEX_POINT" {
				continue
			}
			id := r.Args[1].Ref
			p := f.Entities[id].Records[0]
			if p.Type != "CARTESIAN_POINT" || len(p.Args[1].Items) != 3 {
				t.Fatal("vertex has no original three-dimensional point")
			}
			equal := true
			for axis, v := range p.Args[1].Items {
				rat, ok := new(big.Rat).SetString(v.Text)
				if !ok || rat.Cmp(contact.Point[axis]) != 0 {
					equal = false
				}
			}
			if equal {
				coords[id] = true
				vertexCount++
			}
		}
	}
	if len(coords) != 2 || vertexCount != 2 {
		t.Fatalf("original contact must retain two different STEP point and vertex entities: %d points, %d vertices", len(coords), vertexCount)
	}
}

func checkVariableCutComponents(t *testing.T, ctx context.Context, ext string, raw []byte, reader string, cut *trimmed.ClippedPolynomialTransverseCut) {
	t.Helper()
	if reader == "" {
		return
	}
	file := filepath.Join(t.TempDir(), "variable-cut."+ext)
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(ctx, reader, file).CombinedOutput()
	if err != nil {
		t.Fatalf("independent %s reader failed: %v\n%s", ext, err, out)
	}
	var payload []byte
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if line = bytes.TrimSpace(line); bytes.HasPrefix(line, []byte{'{'}) {
			payload = line
		}
	}
	var actual struct {
		Solids, IndexedVertices int
		Components              []struct {
			Faces, Edges, Shells int
			Volume               float64
			Vertices             [][3]float64
		}
	}
	if err := json.Unmarshal(payload, &actual); err != nil {
		t.Fatalf("independent %s report: %v\n%s", ext, err, out)
	}
	parts := cut.Components()
	volumes, _ := cut.ExactComponentVolumes()
	if actual.Solids != 3 || len(actual.Components) != len(parts) || actual.IndexedVertices != 18 {
		t.Fatalf("independent %s did not retain three separate bodies and 18 original vertices", ext)
	}
	for i, p := range actual.Components {
		v, _ := volumes[i].Float64()
		part := parts[i]
		if p.Shells != 1 || p.Faces != len(part.Faces()) || p.Edges != len(part.Edges()) ||
			p.Volume <= 0 || math.Abs(p.Volume-v) > 1e-9*math.Abs(v) {
			t.Fatalf("independent %s body %d has incorrect topology or signed volume %.16g, want %.16g", ext, i, p.Volume, v)
		}
		want := map[[3]float64]bool{}
		for _, v := range part.Vertices() {
			want[[3]float64{v.X, v.Y, v.Z}] = true
		}
		for _, v := range p.Vertices {
			if !want[v] {
				t.Fatalf("independent %s body %d changed an exact source vertex: %v", ext, i, v)
			}
			delete(want, v)
		}
		if len(want) != 0 {
			t.Fatalf("independent %s body %d omitted source vertices", ext, i)
		}
	}
}
