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
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialThreeTransverseCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, c := range []struct {
		layout string
		solids int
	}{{"partial", 4}, {"dependent", 5}} {
		t.Run(c.layout, func(t *testing.T) {
			f, err := os.Open("../testdata/native_clipped_polynomial_three_cut_" + c.layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			cut, err := trimmed.ReadClippedPolynomialThreeTransverseCutJSON(ctx, f, trimmed.JSONLimits{}, trimmed.PolynomialNodeCutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parts, contacts := cut.Components(), cut.Contacts()
			if len(parts) != c.solids || len(contacts) != 3 {
				t.Fatalf("source has %d solids and %d original contacts", len(parts), len(contacts))
			}
			var raw bytes.Buffer
			opts := ClippedPolynomialCutSTEPOptions{PoleTolerance: 1e-14}
			if err := WriteClippedPolynomialThreeTransverseCut(ctx, &raw, cut, "Three-contact cut", .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			verify := func(p []byte) error {
				return VerifyClippedPolynomialThreeTransverseCutSTEP(ctx, bytes.NewReader(p), cut, "Three-contact cut", .001, Limits{}, opts)
			}
			if err := verify(raw.Bytes()); err != nil {
				t.Fatal(err)
			}
			file, err := Parse(bytes.NewReader(raw.Bytes()), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			pointIDs := map[geom.Vec3][][2]int64{}
			counts := map[string]int{}
			for _, id := range file.Order {
				for _, r := range file.Entities[id].Records {
					counts[r.Type]++
					if r.Type != "VERTEX_POINT" {
						continue
					}
					coord := r.Args[1].Ref
					p, err := mappedExactCoordinate(file, coord, "CARTESIAN_POINT", 3)
					if err != nil {
						t.Fatal(err)
					}
					pointIDs[p] = append(pointIDs[p], [2]int64{id, coord})
				}
			}
			wantFaces, wantVertices := 0, 0
			for _, part := range parts {
				wantFaces += len(part.Faces())
				wantVertices += len(part.Vertices())
			}
			if counts["MANIFOLD_SOLID_BREP"] != c.solids || counts["ADVANCED_FACE"] != wantFaces || counts["VERTEX_POINT"] != wantVertices {
				t.Fatal("STEP welded or dropped source topology", counts)
			}
			var third [2][2]int64
			for i, contact := range contacts {
				p := parts[contact.Components[0]].Vertices()[contact.Vertices[0]]
				found := pointIDs[p]
				if len(found) != 2 || found[0][0] == found[1][0] || found[0][1] == found[1][1] {
					t.Fatalf("contact %d changed vertex occurrence: %v", i, found)
				}
				if i == 2 {
					third = [2][2]int64(found)
				}
			}
			before := []byte(fmt.Sprintf("#%d=VERTEX_POINT('',#%d);", third[1][0], third[1][1]))
			after := []byte(fmt.Sprintf("#%d=VERTEX_POINT('',#%d);", third[1][0], third[0][1]))
			changed := bytes.Replace(raw.Bytes(), before, after, 1)
			if bytes.Equal(raw.Bytes(), changed) {
				t.Fatal("no third-contact tamper exercised")
			}
			if err := verify(changed); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("shared third contact coordinate entity accepted: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_FUSE_STEP"); oracle != "" {
				path := filepath.Join(t.TempDir(), "three-contact-cut.step")
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
				volume, _ := exact.Float64()
				if got.Solids != c.solids || got.Faces != wantFaces || math.Abs(got.Volume-volume) > 1e-7 {
					t.Fatalf("independent STEP measures %+v, exact volume %.16g", got, volume)
				}
			}
		})
	}
}
