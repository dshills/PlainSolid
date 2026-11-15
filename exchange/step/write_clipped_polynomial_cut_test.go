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

func TestWriteClippedPolynomialCut(t *testing.T) {
	ctx := context.Background()
	for layout, count := range map[string]int{"partial": 3, "dependent": 4} {
		t.Run(layout, func(t *testing.T) {
			input, err := os.Open("../testdata/native_clipped_polynomial_cut_" + layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			cut, err := trimmed.ReadClippedPolynomialCutJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialNodeCutOptions{})
			if err != nil || len(cut.Components()) != count {
				t.Fatal("native components", err)
			}
			opts := ClippedPolynomialCutSTEPOptions{PoleTolerance: 1e-14}
			var raw bytes.Buffer
			if err := WriteClippedPolynomialCut(ctx, &raw, cut, "Cut material", .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			file, err := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			actual := map[string]int{}
			points := map[geom.Vec3][][2]int64{}
			for _, e := range file.Entities {
				for _, r := range e.Records {
					actual[r.Type]++
					if r.Type != "VERTEX_POINT" {
						continue
					}
					p, err := mappedExactCoordinate(file, r.Args[1].Ref, "CARTESIAN_POINT", 3)
					if err != nil {
						t.Fatal(err)
					}
					points[p] = append(points[p], [2]int64{e.ID, r.Args[1].Ref})
				}
			}
			want := map[string]int{"MANIFOLD_SOLID_BREP": count, "CLOSED_SHELL": count}
			for _, c := range cut.Components() {
				want["ADVANCED_FACE"] += len(c.Faces())
				want["PCURVE"] += 2 * len(c.Edges())
				want["VERTEX_POINT"] += len(c.Vertices())
			}
			for kind, n := range want {
				if actual[kind] != n {
					t.Fatalf("%s = %d, want %d", kind, actual[kind], n)
				}
			}
			var corrupt []byte
			for _, c := range cut.Contacts() {
				part := cut.Components()[c.Components[0]]
				p := part.Vertices()[c.Vertices[0]]
				ids := points[p]
				if len(ids) != 2 || ids[0][0] == ids[1][0] || ids[0][1] == ids[1][1] {
					t.Fatalf("contact has no independent exact vertex occurrences: %v %v", p, ids)
				}
				// Refer the first contact vertex to an unrelated vertex's
				// coordinate while leaving the original STEP graph parseable.
				for _, v := range part.Vertices() {
					if v == p {
						continue
					}
					from := []byte(fmt.Sprintf("#%d=VERTEX_POINT('',#%d);", ids[0][0], ids[0][1]))
					to := []byte(fmt.Sprintf("#%d=VERTEX_POINT('',#%d);", ids[0][0], points[v][0][1]))
					corrupt = bytes.Replace(raw.Bytes(), from, to, 1)
					break
				}
			}
			if err := VerifyClippedPolynomialCutSTEP(ctx, bytes.NewReader(raw.Bytes()), cut, "Cut material", .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(corrupt, raw.Bytes()) {
				t.Fatal("no contact corruption exercised")
			}
			if err := VerifyClippedPolynomialCutSTEP(ctx, bytes.NewReader(corrupt), cut, "Cut material", .001, Limits{}, opts); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed contact coordinate accepted: %v", err)
			}
			var refused bytes.Buffer
			if err := WriteClippedPolynomialCut(ctx, &refused, cut, "Cut material", .001, Limits{MaxSolids: count - 1}, opts); err == nil || refused.Len() != 0 {
				t.Fatalf("solid limit returned a prefix: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_STEP"); oracle != "" {
				path := filepath.Join(t.TempDir(), "clipped-cut.step")
				if err := os.WriteFile(path, raw.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				data, err := exec.Command(oracle, "read", path).CombinedOutput()
				var got struct {
					Volume        float64
					Faces, Solids int
				}
				if err != nil || json.Unmarshal(data, &got) != nil {
					t.Fatalf("independent STEP reader: %v\n%s", err, data)
				}
				_, exact := cut.ExactComponentVolumes()
				x, _ := exact.Float64()
				if got.Solids != count || got.Faces != want["ADVANCED_FACE"] || math.Abs(got.Volume-x) > 1e-7 {
					t.Fatalf("independent component measures: %+v, exact volume %.16g", got, x)
				}
			}
		})
	}
}
