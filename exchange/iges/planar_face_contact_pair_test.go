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
	"plainsolid/geom"
)

func facePairTube(t *testing.T, z, holeShift float64) brep.Solid {
	t.Helper()
	square := func(lo, hi, shift float64) []geom.Vec3 {
		return []geom.Vec3{{X: lo + shift, Y: lo, Z: z}, {X: hi + shift, Y: lo, Z: z}, {X: hi + shift, Y: hi, Z: z}, {X: lo + shift, Y: hi, Z: z}}
	}
	s, err := brep.PrismWithHoles(square(0, 4, 0), [][]geom.Vec3{square(1, 3, holeShift)}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func checkFacePairTopology(t *testing.T, p collision.PlanarFaceContact, a, b brep.Solid) {
	t.Helper()
	if p.SignedVolume == nil || p.SignedVolume.RatString() != "24" || p.TwiceProjectedArea.RatString() != "24" ||
		len(p.WireOffsets) != 3 || p.WireOffsets[1] != 4 || p.WireOffsets[2] != 8 || len(p.Edges) != 8 ||
		len(a.Faces()) != 10 || len(b.Faces()) != 10 {
		t.Fatalf("annular contact lost original topology: %+v", p)
	}
	for _, v := range p.Vertices {
		if a.Vertices()[v[0]] != b.Vertices()[v[1]] {
			t.Fatalf("paired source vertices differ: %v", v)
		}
	}
	joined, _, err := csg.JoinPlanarFacePair(context.Background(), a, b, p.FaceA, p.FaceB, collision.PlanarFaceContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := joined.MassProperties()
	if err != nil || math.Abs(m.Volume-24) > 1e-10 || math.Abs(m.SurfaceArea-72) > 1e-10 || len(joined.Faces()) != 18 {
		t.Fatalf("joint material no longer matches original bodies: %+v, %d faces, %v", m, len(joined.Faces()), err)
	}
}

func TestPlanarFaceContactPairExchange(t *testing.T) {
	ctx := context.Background()
	a, b := facePairTube(t, 0, 0), facePairTube(t, 1, 0)
	errWrong := collision.ErrInvalid
	{
		var buf bytes.Buffer
		m := step.Model{LengthUnit: .001, Bodies: []step.Body{{Name: "Lower", Solid: a}, {Name: "Upper", Solid: b}}}
		if _, err := step.WriteFaceContactPair(ctx, &buf, m, step.FaceContactSTEPOptions{}); err != nil {
			t.Fatal(err)
		}
		got, p, err := step.ReadFaceContactPair(ctx, bytes.NewReader(buf.Bytes()), step.FaceContactSTEPOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.LengthUnit != m.LengthUnit || got.Bodies[0].Name != "Lower" || got.Bodies[1].Name != "Upper" {
			t.Fatal("STEP changed original units or names")
		}
		checkFacePairTopology(t, p, got.Bodies[0].Solid, got.Bodies[1].Solid)
		if _, p, err := step.ReadEdgeContactPair(ctx, bytes.NewReader(buf.Bytes()), step.EdgeContactSTEPOptions{}); err == nil || p.SignedVolume != nil {
			t.Fatal("edge-only STEP reader admitted original face contact")
		}
		buf.Reset()
		m.Bodies[1].Solid = facePairTube(t, 1, .25)
		if p, err := step.WriteFaceContactPair(ctx, &buf, m, step.FaceContactSTEPOptions{}); !errors.Is(err, errWrong) || p.SignedVolume != nil || buf.Len() != 0 {
			t.Fatalf("STEP published mismatched inner wires: %+v %v", p, err)
		}
	}
	{
		var buf bytes.Buffer
		parts := [2]brep.Solid{a, b}
		if _, err := iges.WritePlanarFaceContactPair(ctx, &buf, parts, .001, iges.FaceContactIGESOptions{}); err != nil {
			t.Fatal(err)
		}
		got, err := iges.ReadPlanarFaceContactPair(ctx, bytes.NewReader(buf.Bytes()), iges.FaceContactIGESOptions{})
		if err != nil {
			t.Fatal(err)
		}
		checkFacePairTopology(t, got.Contact, got.Bodies[0].Solid, got.Bodies[1].Solid)
		if got.LengthUnit != .001 || got.GroupID == 0 || got.Bodies[0].SolidID == got.Bodies[1].SolidID {
			t.Fatal("IGES lost original separate material roots or units")
		}
		for _, q := range got.Contact.Vertices {
			if got.Bodies[0].SourceVertexIndices[q[0]] == got.Bodies[1].SourceVertexIndices[q[1]] {
				t.Fatal("IGES welded an original shared-face vertex")
			}
		}
		if got, err := iges.ReadPlanarEdgeContactPair(ctx, bytes.NewReader(buf.Bytes()), iges.EdgeContactIGESOptions{}); err == nil || got.Contact.SignedVolume != nil {
			t.Fatal("edge-only IGES reader admitted original face contact")
		}
		buf.Reset()
		if p, err := iges.WritePlanarFaceContactPair(ctx, &buf, parts, .001, iges.FaceContactIGESOptions{Proof: collision.PlanarFaceDiscoveryOptions{MaxBoundaryChecks: 1}}); !errors.Is(err, collision.ErrLimit) || p.SignedVolume != nil || buf.Len() != 0 {
			t.Fatalf("IGES published a budget-limited match: %+v %v", p, err)
		}
	}
}

func TestPlanarFaceContactPairExchangeLiveOCCT(t *testing.T) {
	for _, format := range []string{"STEP", "IGES"} {
		t.Run(format, func(t *testing.T) {
			ctx := context.Background()
			igesTool := os.Getenv("PLAINSOLID_OCCT_PLANAR_FACE_IGES")
			stepTool := os.Getenv("PLAINSOLID_OCCT_PLANAR_FACE_STEP")
			stepReader := os.Getenv("PLAINSOLID_OCCT_VERTEX_CONTACT_STEP")
			author, reader, mode := stepTool, stepReader, "face-annulus"
			if format == "IGES" {
				author, reader, mode = igesTool, igesTool, "write-face-annulus"
			}
			if author == "" || reader == "" {
				t.Skip("optional pinned OCCT face-contact author and reader")
			}
			dir := t.TempDir()
			upstream, native := filepath.Join(dir, "original."+format), filepath.Join(dir, "native."+format)
			if output, err := exec.Command(author, mode, upstream).CombinedOutput(); err != nil {
				t.Fatalf("OCCT author: %v %s", err, output)
			}
			raw, err := os.ReadFile(upstream)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if format == "STEP" {
				m, p, err := step.ReadFaceContactPair(ctx, bytes.NewReader(raw), step.FaceContactSTEPOptions{})
				if err != nil {
					t.Fatal(err)
				}
				checkFacePairTopology(t, p, m.Bodies[0].Solid, m.Bodies[1].Solid)
				if _, err := step.WriteFaceContactPair(ctx, &buf, m, step.FaceContactSTEPOptions{}); err != nil {
					t.Fatal(err)
				}
			} else {
				m, err := iges.ReadPlanarFaceContactPair(ctx, bytes.NewReader(raw), iges.FaceContactIGESOptions{})
				if err != nil {
					t.Fatal(err)
				}
				a, b := m.Bodies[0].Solid, m.Bodies[1].Solid
				checkFacePairTopology(t, m.Contact, a, b)
				if _, err := iges.WritePlanarFaceContactPair(ctx, &buf, [2]brep.Solid{a, b}, m.LengthUnit, iges.FaceContactIGESOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(native, buf.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{upstream, native} {
				data, err := exec.Command(reader, "read", path).CombinedOutput()
				var r struct {
					Solids, Faces int
					Volume, Area  float64
				}
				i := bytes.LastIndexByte(data, '{')
				if err != nil || i < 0 || json.Unmarshal(data[i:], &r) != nil || r.Solids != 2 || r.Faces != 20 || math.Abs(r.Volume-24) > 1e-9 || math.Abs(r.Area-96) > 1e-9 {
					t.Fatalf("OCCT readback changed separate solids: %s: %v %s", path, err, data)
				}
			}
		})
	}
}
