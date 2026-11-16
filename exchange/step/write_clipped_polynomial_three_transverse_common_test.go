package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialThreeTransverseCommon(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name         string
		volume, area float64
	}{
		{"partial", 26955626929285653.0 / 2251799813685248.0, 32.985201663206595},
		{"dependent", 6580961624997.0 / 549755813888.0, 31.655074821630230},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile("../testdata/native_clipped_polynomial_three_common_" + tc.name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			common, err := trimmed.ReadClippedPolynomialThreeTransverseCommonJSON(ctx, bytes.NewReader(data), trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			const name = "Three-contact Common"
			opts := ClippedPolynomialCommonSTEPOptions{PoleTolerance: 1e-14}
			var buf bytes.Buffer
			if err := WriteClippedPolynomialThreeTransverseCommonSTEP(ctx, &buf, common, name, .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			verify := func(p []byte) error {
				return VerifyClippedPolynomialThreeTransverseCommonSTEP(ctx, bytes.NewReader(p), common, name, .001, Limits{}, opts)
			}
			if err := verify(buf.Bytes()); err != nil {
				t.Fatal(err)
			}
			f, err := Parse(bytes.NewReader(buf.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			solid := common.Boundary()
			contacts := common.Contacts()
			var vertex, coord [3]int64
			counts, incidence := map[string]int{}, map[int64]int{}
			for id, e := range f.Entities {
				for _, r := range e.Records {
					counts[r.Type]++
					switch r.Type {
					case "VERTEX_POINT":
						coordID := r.Args[1].Ref
						p, err := mappedRationalCoordinate(f, coordID, "CARTESIAN_POINT", 3)
						if err != nil {
							t.Fatal(err)
						}
						for i, c := range contacts {
							if p[0].Cmp(c.Point[0]) == 0 && p[1].Cmp(c.Point[1]) == 0 && p[2].Cmp(c.Point[2]) == 0 {
								if vertex[i] != 0 {
									t.Fatalf("split native contact %d into multiple STEP vertices", i)
								}
								vertex[i], coord[i] = id, coordID
							}
						}
					case "EDGE_CURVE":
						incidence[r.Args[1].Ref]++
						incidence[r.Args[2].Ref]++
					}
				}
			}
			if counts["MANIFOLD_SOLID_BREP"] != 1 || counts["ADVANCED_FACE"] != len(solid.Faces()) ||
				counts["VERTEX_POINT"] != len(solid.Vertices()) || counts["PCURVE"] != 2*len(solid.Edges()) {
				t.Fatalf("incomplete original topology or owner pcurves: %v", counts)
			}
			for i, id := range vertex {
				if id == 0 || incidence[id] != 4 {
					t.Fatalf("contact %d must retain four original edge rays: vertex %d, incidence %d", i, id, incidence[id])
				}
			}
			before := []byte(fmt.Sprintf("#%d=VERTEX_POINT('',#%d);", vertex[2], coord[2]))
			after := []byte(fmt.Sprintf("#%d=VERTEX_POINT('',#%d);", vertex[2], coord[0]))
			modified := bytes.Replace(buf.Bytes(), before, after, 1)
			if bytes.Equal(modified, buf.Bytes()) {
				t.Fatal("third-contact tamper did not change the STEP graph")
			}
			if err := verify(modified); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed third contact accepted: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_STEP"); oracle != "" {
				path := filepath.Join(t.TempDir(), "three-contact-common.step")
				if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				out, err := exec.CommandContext(ctx, oracle, "read", path).CombinedOutput()
				var got struct {
					Volume, Area  float64
					Faces, Solids int
				}
				if err != nil || json.Unmarshal(out, &got) != nil {
					t.Fatalf("independent STEP reader: %v\n%s", err, out)
				}
				if got.Solids != 1 || got.Faces != len(solid.Faces()) ||
					math.Abs(got.Volume-tc.volume) > 1e-7 || math.Abs(got.Area-tc.area) > 1e-7 {
					t.Fatalf("independent STEP material: %+v, want %.16g volume and %.16g area", got, tc.volume, tc.area)
				}
				t.Logf("pinned OCCT: %d faces, volume %.16g, area %.16g", got.Faces, got.Volume, got.Area)
			}
		})
	}
}
