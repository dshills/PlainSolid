package step

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
	"plainsolid/geom"
)

const planarConcaveVoidFixture = "testdata/occt-planar-concave-voids.step"

func exactSTEPVoidOptions() VertexContactSTEPOptions {
	return VertexContactSTEPOptions{Proof: collision.VertexContactAssemblyOptions{AllowCavities: true}}
}

func checkExactSTEPVoidSource(t *testing.T, raw []byte) Model {
	t.Helper()
	ctx := context.Background()
	m, proof, err := ReadVertexContactAssembly(ctx, bytes.NewReader(raw), exactSTEPVoidOptions())
	if err != nil || len(m.Bodies) != 2 || m.LengthUnit != .001 || proof.SignedVolume.RatString() != "441" || len(proof.Contacts) != 0 {
		t.Fatalf("exact planar void source: %+v %v", proof, err)
	}
	if top := m.Bodies[0].Solid.Topology(); top.Shells != 2 || top.Faces != 14 {
		t.Fatalf("lost nonconvex exterior/void incidence: %+v", top)
	}
	return m
}

func TestPlanarVertexContactAssemblySTEPConcaveVoid(t *testing.T) {
	src, err := os.ReadFile(planarConcaveVoidFixture)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m := checkExactSTEPVoidSource(t, src)
	if raw, proof, err := ReadVertexContactAssembly(ctx, bytes.NewReader(src), VertexContactSTEPOptions{}); err == nil || raw.Bodies != nil || proof.SignedVolume != nil {
		t.Fatalf("ordinary exact transfer admitted a cavity without opt-in: %v", err)
	}
	var canonical bytes.Buffer
	o := exactSTEPVoidOptions()
	if _, err := WriteVertexContactAssembly(ctx, &canonical, m, o); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(canonical.Bytes(), []byte("BREP_WITH_VOIDS")) || !bytes.Contains(canonical.Bytes(), []byte("ORIENTED_CLOSED_SHELL")) {
		t.Fatal("canonical STEP lost the standard void wrapper")
	}
	checkExactSTEPVoidSource(t, canonical.Bytes())
	m.LengthUnit = .0254
	var inch bytes.Buffer
	if _, err := WriteVertexContactAssembly(ctx, &inch, m, o); err != nil || !bytes.Contains(inch.Bytes(), []byte("LENGTH_MEASURE(0.0254)")) {
		t.Fatalf("exact composed cavity lost its physical inch declaration: %v", err)
	}
	if s, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(inch.Bytes()), o); err != nil || s.LengthUnit != .0254 || p.SignedVolume.RatString() != "441" {
		t.Fatalf("inch cavity did not strictly reopen in original units: %v", err)
	}
	m.LengthUnit = .001
	for _, change := range [][2][]byte{
		{[]byte("#231 = ORIENTED_CLOSED_SHELL('',*,#232,.F.);"), []byte("#231 = ORIENTED_CLOSED_SHELL('',*,#232,.T.);")},
		{[]byte("#231 = ORIENTED_CLOSED_SHELL('',*,#232,.F.);"), []byte("#231 = ORIENTED_CLOSED_SHELL('',*,#16,.F.);")},
	} {
		mutant := bytes.Replace(src, change[0], change[1], 1)
		if bytes.Equal(src, mutant) {
			t.Fatal("mutation did not address the actual source void wrapper")
		}
		if r, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(mutant), o); err == nil || r.Bodies != nil || p.SignedVolume != nil {
			t.Fatalf("changed original shell relations yielded a partial body: %+v %v", p, err)
		}
	}
	l := o
	l.Limits.MaxFaces = 19
	if r, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(src), l); err == nil || r.Bodies != nil || p.SignedVolume != nil {
		t.Fatalf("combined body and cavity limits were not checked: %v", err)
	}
	for _, fail := range []VertexContactSTEPOptions{{}, {Proof: o.Proof, FacetedWriter: true}} {
		var out bytes.Buffer
		if p, err := WriteVertexContactAssembly(ctx, &out, m, fail); err == nil || p.SignedVolume != nil || out.Len() != 0 {
			t.Fatalf("unsupported cavity output yielded STEP bytes: %+v %v", p, err)
		}
	}
}

func TestPlanarVertexContactAssemblySTEPVoidContact(t *testing.T) {
	raw, err := os.ReadFile(planarConcaveVoidFixture)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m := checkExactSTEPVoidSource(t, raw)
	cube, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	v := cube.Vertices()
	for i, p := range v {
		v[i] = geom.Vec3{X: 2 + (p.X+p.Y+p.Z)/4, Y: 2 + (p.X+2*p.Y+p.Z)/4, Z: 2 + (p.X+p.Y+2*p.Z)/4}
	}
	s, err := brep.NewSolid(v, cube.Edges(), cube.Faces())
	if err != nil {
		t.Fatal(err)
	}
	m.Bodies = append(m.Bodies, Body{Name: "Oblique insert", Solid: s})
	o := exactSTEPVoidOptions()
	var out bytes.Buffer
	if _, err := WriteVertexContactAssembly(ctx, &out, m, o); err != nil {
		t.Fatal(err)
	}
	reloaded, ledger, err := ReadVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), o)
	if err != nil || len(reloaded.Bodies) != 3 || ledger.SignedVolume.RatString() != "28225/64" || len(ledger.Contacts) != 1 {
		t.Fatalf("void and third solid no longer have exact original contact: %+v %v", ledger, err)
	}
	point := geom.Vec3{X: 2, Y: 2, Z: 2}
	c := ledger.Contacts[0]
	if c.A.Solid != 0 || c.B.Solid != 2 ||
		reloaded.Bodies[0].Solid.Vertices()[c.A.Vertex] != point ||
		reloaded.Bodies[2].Solid.Vertices()[c.B.Vertex] != point {
		t.Fatalf("cavity contact moved or changed owner: %+v", c)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_VERTEX_CONTACT_STEP"); bin != "" {
		path := filepath.Join(t.TempDir(), "void-contact.step")
		if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.Command(bin, "read", path).CombinedOutput()
		var prop struct {
			Solids, Faces int
			Area, Volume  float64
		}
		if err != nil || json.Unmarshal(data, &prop) != nil || prop.Solids != 3 || prop.Faces != 26 ||
			math.Abs(prop.Volume-28225./64) > 1e-9 || math.Abs(prop.Area-(398+(math.Sqrt2+math.Sqrt(11)/2)/4)) > 1e-9 {
			t.Fatalf("pinned OCCT did not recover the same material: %v %s", err, data)
		}
	}
}

func TestPlanarVertexContactAssemblySTEPConcaveVoidLive(t *testing.T) {
	author := os.Getenv("PLAINSOLID_OCCT_PLANAR_CONTACT_AUTHOR")
	if author == "" {
		t.Skip("optional pinned OCCT nonconvex planar cavity author")
	}
	path := filepath.Join(t.TempDir(), "original-concave-void.step")
	if out, err := exec.Command(author, "voids", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %s %v", out, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checkExactSTEPVoidSource(t, raw)
}
