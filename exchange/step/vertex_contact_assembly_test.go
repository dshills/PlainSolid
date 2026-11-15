package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func vertexContactSTEPFixture(t *testing.T) Model {
	t.Helper()
	m := Model{LengthUnit: .001}
	for i, span := range [][2]float64{{0, 1}, {1, 2}, {4, 5}} {
		xyz := func(v float64) geom.Vec3 { return geom.Vec3{X: v, Y: v, Z: v} }
		s, err := brep.Box(xyz(span[0]), xyz(span[1]))
		if err != nil {
			t.Fatal(err)
		}
		m.Bodies = append(m.Bodies, Body{Name: []string{"First", "Second", "Separate"}[i], Solid: s})
	}
	return m
}

func TestPlanarVertexContactAssemblySTEP(t *testing.T) {
	ctx := context.Background()
	m := vertexContactSTEPFixture(t)
	for _, faceted := range []bool{false, true} {
		o := VertexContactSTEPOptions{FacetedWriter: faceted}
		var out bytes.Buffer
		w, err := WriteVertexContactAssembly(ctx, &out, m, o)
		if err != nil || len(w.Contacts) != 1 || w.SignedVolume.Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatalf("faceted=%v write %+v: %v", faceted, w, err)
		}
		r, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), o)
		if err != nil || len(r.Bodies) != 3 || len(p.Contacts) != 1 || p.SignedVolume.Cmp(big.NewRat(3, 1)) != 0 || r.LengthUnit != m.LengthUnit {
			t.Fatalf("faceted=%v read %+v: %v", faceted, p, err)
		}
		c := p.Contacts[0]
		if c.A.Solid != 0 || c.B.Solid != 1 || r.Bodies[0].Solid.Vertices()[c.A.Vertex] != (geom.Vec3{X: 1, Y: 1, Z: 1}) || r.Bodies[1].Solid.Vertices()[c.B.Vertex] != (geom.Vec3{X: 1, Y: 1, Z: 1}) {
			t.Fatalf("faceted=%v lost original contact %+v", faceted, c)
		}
		limited := VertexContactSTEPOptions{Limits: Limits{MaxBytes: 64}}
		var truncated bytes.Buffer
		if _, err := WriteVertexContactAssembly(ctx, &truncated, m, limited); !errors.Is(err, collision.ErrLimit) || truncated.Len() != 0 {
			t.Fatalf("bounded writer published incomplete bytes: %v", err)
		}
	}
}

func TestPlanarVertexContactAssemblySTEPRejectsEdgeOverlapAndContainment(t *testing.T) {
	ctx := context.Background()
	for _, interval := range [][2]geom.Vec3{
		{{X: 1, Y: 1}, {X: 2, Y: 2, Z: 1}},
		{{X: .5, Y: .5, Z: .5}, {X: 2, Y: 2, Z: 2}},
		{{X: .25, Y: .25, Z: .25}, {X: .75, Y: .75, Z: .75}},
	} {
		m := vertexContactSTEPFixture(t)
		body, err := brep.Box(interval[0], interval[1])
		if err != nil {
			t.Fatal(err)
		}
		m.Bodies[1].Solid = body
		var independent, rejected bytes.Buffer
		if err := WriteAdvanced(&independent, m); err != nil {
			t.Fatal(err)
		}
		if r, p, err := ReadVertexContactAssembly(ctx, &independent, VertexContactSTEPOptions{}); !errors.Is(err, collision.ErrInvalid) || r.Bodies != nil || p.SignedVolume != nil {
			t.Fatalf("invalid contact yielded partial model: %v", err)
		}
		if p, err := WriteVertexContactAssembly(ctx, &rejected, m, VertexContactSTEPOptions{}); !errors.Is(err, collision.ErrInvalid) || rejected.Len() != 0 || p.SignedVolume != nil {
			t.Fatalf("invalid contact wrote an output prefix: %v", err)
		}
	}
}

func TestPlanarVertexContactAssemblySTEPOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_VERTEX_CONTACT_STEP")
	if bin == "" {
		t.Skip("optional pinned OCCT standard STEP reader")
	}
	ctx := context.Background()
	var step bytes.Buffer
	if _, err := WriteVertexContactAssembly(ctx, &step, vertexContactSTEPFixture(t), VertexContactSTEPOptions{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "planar-vertex-contact.step")
	if err := os.WriteFile(path, step.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT STEP transfer: %v (%s)", err, out)
	}
	var p struct {
		Solids, Faces int
		Volume, Area  float64
		Centroid      [3]float64
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &p); err != nil || p.Solids != 3 || p.Faces != 18 || math.Abs(p.Volume-3) > 1e-9 || math.Abs(p.Area-18) > 1e-9 {
		t.Fatalf("pinned OCCT measured %+v from %q (%v)", p, out, err)
	}
}
