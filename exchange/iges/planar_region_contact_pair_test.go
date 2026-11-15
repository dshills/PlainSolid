package iges_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/csg"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
)

func TestPlanarRegionContactPairExchangeLiveOCCT(t *testing.T) {
	for _, format := range []string{"step", "igs"} {
		t.Run(format, func(t *testing.T) {
			ctx := context.Background()
			var author, reader, mode string
			switch format {
			case "step":
				author = os.Getenv("PLAINSOLID_OCCT_PLANAR_REGION_STEP")
				reader = os.Getenv("PLAINSOLID_OCCT_VERTEX_CONTACT_STEP")
				mode = "face-subdivided"
			case "igs":
				author = os.Getenv("PLAINSOLID_OCCT_PLANAR_REGION_IGES")
				reader = author
				mode = "write-face-subdivided"
			}
			if author == "" || reader == "" {
				t.Skip("optional pinned independent planar region tools")
			}
			dir := t.TempDir()
			upstream, native := filepath.Join(dir, "original."+format), filepath.Join(dir, "native."+format)
			if d, e := exec.Command(author, mode, upstream).CombinedOutput(); e != nil {
				t.Fatalf("OCCT source: %v %s", e, d)
			}
			raw, err := os.ReadFile(upstream)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			var p collision.PlanarRegionContact
			var parts [2]brep.Solid
			if format == "step" {
				m, q, err := step.ReadPlanarRegionContactPair(ctx, bytes.NewReader(raw), step.PlanarRegionSTEPOptions{})
				if err != nil {
					t.Fatal(err)
				}
				p = q
				parts = [2]brep.Solid{m.Bodies[0].Solid, m.Bodies[1].Solid}
				if _, err := step.WritePlanarRegionContactPair(ctx, &buf, m, step.PlanarRegionSTEPOptions{}); err != nil {
					t.Fatal(err)
				}
				if _, p, err := step.ReadFaceContactPair(ctx, bytes.NewReader(buf.Bytes()), step.FaceContactSTEPOptions{}); err == nil || p.SignedVolume != nil {
					t.Fatal("single original face reader accepted the independent four-way subdivision")
				}
			} else {
				m, err := iges.ReadPlanarRegionContactPair(ctx, bytes.NewReader(raw), iges.PlanarRegionIGESOptions{})
				if err != nil {
					t.Fatal(err)
				}
				p = m.Contact
				parts = [2]brep.Solid{m.Bodies[0].Solid, m.Bodies[1].Solid}
				for _, v := range p.Vertices {
					if m.Bodies[0].SourceVertexIndices[v[0]] == m.Bodies[1].SourceVertexIndices[v[1]] {
						t.Fatal("IGES shared a source vertex ID across distinct parts")
					}
				}
				if _, err := iges.WritePlanarRegionContactPair(ctx, &buf, parts, m.LengthUnit, iges.PlanarRegionIGESOptions{}); err != nil {
					t.Fatal(err)
				}
				if m, err := iges.ReadPlanarFaceContactPair(ctx, bytes.NewReader(buf.Bytes()), iges.FaceContactIGESOptions{}); err == nil || m.Contact.SignedVolume != nil {
					t.Fatal("single original IGES face reader accepted the independent four-way subdivision")
				}
			}
			a, b := len(p.Faces[0]), len(p.Faces[1])
			if p.SignedVolume.RatString() != "96" || len(p.Edges) != 4 || len(p.Vertices) != 4 || min(a, b) != 1 || max(a, b) != 4 {
				t.Fatalf("lost independent original face divisions: %+v", p)
			}
			s, _, err := csg.JoinPlanarRegions(ctx, parts[0], parts[1], p.Faces, collision.PlanarFaceContactOptions{})
			if err != nil {
				t.Fatal(err)
			}
			properties, err := s.MassProperties()
			if err != nil || len(s.Faces()) != 10 || math.Abs(properties.SurfaceArea-152) > 1e-9 {
				t.Fatalf("wrong joined material: %+v %v", properties, err)
			}
			if err := os.WriteFile(native, buf.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{upstream, native} {
				data, err := exec.Command(reader, "read", path).CombinedOutput()
				i := bytes.LastIndexByte(data, '{')
				var q struct {
					Solids, Faces int
					Volume, Area  float64
				}
				if err != nil || i < 0 || json.Unmarshal(data[i:], &q) != nil || q.Solids != 2 || q.Faces != 15 || math.Abs(q.Volume-96) > 1e-9 || math.Abs(q.Area-248) > 1e-9 {
					t.Fatalf("independent OCCT readback changed source topology: %s: %v %s", path, err, data)
				}
			}
		})
	}
}
