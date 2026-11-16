package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialCommon(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"mixed", "transverse"} {
		t.Run(kind, func(t *testing.T) {
			input, err := os.Open("../testdata/native_clipped_polynomial_common_" + kind + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			source, err := trimmed.ReadClippedPolynomialCommonJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			s := source.Boundary()
			var raw bytes.Buffer
			opts := ClippedPolynomialCommonSTEPOptions{PoleTolerance: 1e-14}
			if kind == "mixed" {
				// The native mixed-contact material is valid, but pinned
				// OCCT rejects the narrow high-order cusp in the raw transfer.
				if err := WriteClippedPolynomialCommon(ctx, &raw, source, "Clipped Common", .001, Limits{}, opts); !errors.Is(err, geom.ErrDomain) || raw.Len() != 0 {
					t.Fatalf("unsupported cusp published bytes: %v", err)
				}
				return
			}
			if err := WriteClippedPolynomialCommon(ctx, &raw, source, "Clipped Common", .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			verify := func(data []byte) error {
				return VerifyClippedPolynomialCommonSTEP(ctx, bytes.NewReader(data), source, "Clipped Common", .001, Limits{}, opts)
			}
			if err := verify(raw.Bytes()); err != nil {
				t.Fatal(err)
			}
			modified := bytes.Replace(raw.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S2."), 1)
			if err := verify(modified); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed original curve master accepted: %v", err)
			}
			var refused bytes.Buffer
			if err := WriteClippedPolynomialCommon(ctx, &refused, &trimmed.AlignedPolynomialEnvelope{}, "", .001, Limits{}, opts); !errors.Is(err, geom.ErrInvalid) || refused.Len() != 0 {
				t.Fatalf("unproved input published bytes: %v", err)
			}
			file, err := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			for _, e := range file.Entities {
				for _, r := range e.Records {
					counts[r.Type]++
				}
			}
			if counts["MANIFOLD_SOLID_BREP"] != 1 || counts["ADVANCED_FACE"] != len(s.Faces()) || counts["PCURVE"] != 2*len(s.Edges()) {
				t.Fatal("a wall, model edge or original coedge was lost", counts)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_STEP"); oracle != "" {
				path := filepath.Join(t.TempDir(), "clipped-common.step")
				if e := os.WriteFile(path, raw.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				data, err := exec.Command(oracle, "read", path).CombinedOutput()
				var got struct {
					Volume, Area  float64
					Solids, Faces int
				}
				if err != nil || json.Unmarshal(data, &got) != nil {
					t.Fatalf("independent STEP reader: %v\n%s", err, data)
				}
				want, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil || got.Solids != 1 || got.Faces != len(s.Faces()) || math.Abs(got.Volume-want.Volume) > 1e-7 || math.Abs(got.Area-want.Area) > 1e-7 {
					t.Fatalf("independent STEP measures: %+v, native volume %.16g area %.16g: %v", got, want.Volume, want.Area, err)
				}
			}
		})
	}
}
