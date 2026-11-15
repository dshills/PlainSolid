package iges_test

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

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/csg"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
)

func TestPlanarInsetContactPairExchangeLiveOCCT(t *testing.T) {
	for _, format := range []string{"step", "igs"} {
		t.Run(format, func(t *testing.T) {
			ctx := context.Background()
			var author, reader, mode string
			if format == "step" {
				author = os.Getenv("PLAINSOLID_OCCT_PLANAR_INSET_STEP")
				reader = os.Getenv("PLAINSOLID_OCCT_VERTEX_CONTACT_STEP")
				mode = "face-insets"
			} else {
				author = os.Getenv("PLAINSOLID_OCCT_PLANAR_INSET_IGES")
				reader = author
				mode = "write-face-insets"
			}
			if author == "" || reader == "" {
				t.Skip("optional pinned OCCT author and reader")
			}
			dir := t.TempDir()
			source, output := filepath.Join(dir, "source."+format), filepath.Join(dir, "output."+format)
			if data, err := exec.Command(author, mode, source).CombinedOutput(); err != nil {
				t.Fatalf("original OCCT inset parts: %v %s", err, data)
			}
			raw, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			var p collision.PlanarInsetContactSet
			var parts [2]brep.Solid
			var out bytes.Buffer
			if format == "step" {
				m, proof, err := step.ReadPlanarInsetContactPair(ctx, bytes.NewReader(raw), step.PlanarInsetSTEPOptions{})
				if err != nil {
					t.Fatal(err)
				}
				p = proof
				parts = [2]brep.Solid{m.Bodies[0].Solid, m.Bodies[1].Solid}
				if _, err := step.WritePlanarInsetContactPair(ctx, &out, m, step.PlanarInsetSTEPOptions{}); err != nil {
					t.Fatal(err)
				}
				m.Bodies[0], m.Bodies[1] = m.Bodies[1], m.Bodies[0]
				var bad bytes.Buffer
				if p, err := step.WritePlanarInsetContactPair(ctx, &bad, m, step.PlanarInsetSTEPOptions{}); !errors.Is(err, collision.ErrInvalid) || bad.Len() != 0 || p.SignedVolume != nil {
					t.Fatalf("wrote tool as host: %+v %v", p, err)
				}
			} else {
				m, err := iges.ReadPlanarInsetContactPair(ctx, bytes.NewReader(raw), iges.PlanarInsetIGESOptions{})
				if err != nil {
					t.Fatal(err)
				}
				p = m.Contact
				parts = [2]brep.Solid{m.Bodies[0].Solid, m.Bodies[1].Solid}
				if m.Bodies[0].SolidID == m.Bodies[1].SolidID || m.GroupID == 0 {
					t.Fatal("IGES lost the independent original bodies")
				}
				if _, err := iges.WritePlanarInsetContactPair(ctx, &out, parts, m.LengthUnit, iges.PlanarInsetIGESOptions{}); err != nil {
					t.Fatal(err)
				}
				var bad bytes.Buffer
				if p, err := iges.WritePlanarInsetContactPair(ctx, &bad, [2]brep.Solid{parts[1], parts[0]}, m.LengthUnit, iges.PlanarInsetIGESOptions{}); !errors.Is(err, collision.ErrInvalid) || bad.Len() != 0 || p.SignedVolume != nil {
					t.Fatalf("wrote tool as host: %+v %v", p, err)
				}
			}
			if p.SignedVolume.RatString() != "25" || len(p.Tools) != 2 {
				t.Fatalf("original inset contacts changed: %+v", p)
			}
			for _, face := range p.Tools {
				if face.TwiceToolArea.RatString() != "2" || face.TwiceHostArea.RatString() != "40" || len(face.ToolEdges) != 4 {
					t.Fatalf("lost a complete original tool footprint: %+v", face)
				}
			}
			body, _, err := csg.JoinCoincidentPlanarInsets(ctx, parts[0], parts[1], collision.PlanarFaceContactOptions{})
			if err != nil {
				t.Fatal(err)
			}
			mass, err := body.MassProperties()
			if err != nil || len(body.Faces()) != 14 || math.Abs(mass.Volume-25) > 1e-9 || math.Abs(mass.SurfaceArea-76) > 1e-9 {
				t.Fatalf("source-derived inset join changed exact material: %+v %v", mass, err)
			}
			if err := os.WriteFile(output, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{source, output} {
				data, err := exec.Command(reader, "read", path).CombinedOutput()
				i := bytes.LastIndexByte(data, '{')
				var q struct {
					Solids, Faces int
					Volume, Area  float64
				}
				if err != nil || i < 0 || json.Unmarshal(data[i:], &q) != nil || q.Solids != 2 || q.Faces != 16 ||
					math.Abs(q.Volume-25) > 1e-9 || math.Abs(q.Area-80) > 1e-9 {
					t.Fatalf("independent OCCT readback changed: %s: %v %s", path, err, data)
				}
			}
		})
	}
}
