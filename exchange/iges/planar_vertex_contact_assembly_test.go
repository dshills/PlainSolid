package iges

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
	"plainsolid/geom"
)

const planarAssemblyFixture = "testdata/occt-planar-vertex-contact-assembly.igs"

func checkPlanarVertexContactIGES(t *testing.T, b []byte) (PlanarVertexContactAssembly, []brep.Solid) {
	t.Helper()
	m, err := ReadPlanarVertexContactAssembly(context.Background(), bytes.NewReader(b), PlanarVertexContactAssemblyOptions{})
	if err != nil || len(m.Bodies) != 3 || m.LengthUnit != .001 || m.Proof.SignedVolume.RatString() != "3" || len(m.Proof.Contacts) != 1 {
		t.Fatalf("independent exact planar IGES assembly: %+v %v", m.Proof, err)
	}
	solids := make([]brep.Solid, len(m.Bodies))
	for i, body := range m.Bodies {
		solids[i] = body.Solid
		if top := body.Solid.Topology(); top.Faces != 6 || top.Edges != 12 || top.Vertices != 8 ||
			len(body.SourceVertexIndices) != 8 || len(body.SourceEdgeIndices) != 12 || len(body.SourceEdges) != 12 {
			t.Fatalf("incorrect source topology: %+v", body)
		}
	}
	c := m.Proof.Contacts[0]
	if c.A.Solid != 0 || c.B.Solid != 1 {
		t.Fatalf("unexpected component contact: %+v", c)
	}
	a, b2 := m.Bodies[c.A.Solid], m.Bodies[c.B.Solid]
	p := geom.Vec3{X: 1, Y: 1, Z: 1}
	if a.Solid.Vertices()[c.A.Vertex] != p || b2.Solid.Vertices()[c.B.Vertex] != p ||
		a.SourceVertexIndices[c.A.Vertex] == b2.SourceVertexIndices[c.B.Vertex] {
		t.Fatalf("contact must refer to distinct source vertices at %v: %+v", p, c)
	}
	return m, solids
}

func TestPlanarVertexContactAssemblyIGES(t *testing.T) {
	src, err := os.ReadFile(planarAssemblyFixture)
	if err != nil {
		t.Fatal(err)
	}
	m, solids := checkPlanarVertexContactIGES(t, src)
	c := m.Proof.Contacts[0]
	if m.GroupID != 1 || m.Bodies[0].SolidID != 3 || m.Bodies[1].SolidID != 155 || m.Bodies[2].SolidID != 231 ||
		m.Bodies[0].SourceVertexIndices[c.A.Vertex] != 8 || m.Bodies[1].SourceVertexIndices[c.B.Vertex] != 9 {
		t.Fatalf("the independently authored group and record identity were lost: %+v", m)
	}
	ctx := context.Background()
	var output, repeated bytes.Buffer
	o := PlanarVertexContactAssemblyOptions{}
	if _, err := WritePlanarVertexContactAssembly(ctx, &output, solids, m.LengthUnit, o); err != nil {
		t.Fatal(err)
	}
	_, reloaded := checkPlanarVertexContactIGES(t, output.Bytes())
	if _, err := WritePlanarVertexContactAssembly(ctx, &repeated, reloaded, m.LengthUnit, o); err != nil || !bytes.Equal(output.Bytes(), repeated.Bytes()) {
		t.Fatalf("canonical assembly IGES was not byte stable: %v", err)
	}
	if _, err := ReadPlanarSolid(ctx, bytes.NewReader(src), PlanarSolidOptions{}); err == nil {
		t.Fatal("one-root entry point silently consumed multiple independent bodies")
	}
}

func TestPlanarVertexContactAssemblyIGESRejectsChangedOwnership(t *testing.T) {
	raw, err := os.ReadFile(planarAssemblyFixture)
	if err != nil {
		t.Fatal(err)
	}
	_, original := checkPlanarVertexContactIGES(t, raw)
	welded := editPlanarIGESParameters(t, raw, func(r topologyRecord) bool {
		if r.kind != 504 {
			return false
		}
		mutated := false
		for i := 2; i+4 < len(r.values); i += 5 {
			for _, offset := range []int{2, 4} {
				if r.values[i+offset].text == "9" {
					r.values[i+offset].text = "8"
					mutated = true
				}
			}
		}
		return mutated
	})
	omitted := makePlanarIGESMutation(t, raw, 1, 4, "155")
	ctx := context.Background()
	o := PlanarVertexContactAssemblyOptions{}
	for _, input := range [][]byte{welded, omitted} {
		m, err := ReadPlanarVertexContactAssembly(ctx, bytes.NewReader(input), o)
		if err == nil || m.GroupID != 0 || m.Bodies != nil || m.Proof.SignedVolume != nil {
			t.Fatalf("corrupt body ownership yielded a partial exact assembly: %+v %v", m, err)
		}
	}
	for _, low := range []geom.Vec3{{X: 1, Y: 1, Z: .5}, {X: 1, Y: .5, Z: .5}, {X: .5, Y: .5, Z: .5}} {
		b, err := brep.Box(low, low.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		p, err := WritePlanarVertexContactAssembly(ctx, &out, []brep.Solid{original[0], b}, .001, o)
		if !errors.Is(err, collision.ErrInvalid) || p.SignedVolume != nil || out.Len() != 0 {
			t.Fatalf("non-vertex contact escaped the exact writer: %v", err)
		}
	}
	var out bytes.Buffer
	o.Proof.MaxSolids = 2
	if _, err := WritePlanarVertexContactAssembly(ctx, &out, original, .001, o); !errors.Is(err, collision.ErrLimit) || out.Len() != 0 {
		t.Fatalf("bounded writer emitted partial output: %v", err)
	}
}

func TestPlanarVertexContactAssemblyIGESLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES")
	if bin == "" {
		t.Skip("optional pinned OCCT IGES assembly author and reader")
	}
	dir := t.TempDir()
	upstream, native := filepath.Join(dir, "reference.igs"), filepath.Join(dir, "native.igs")
	if data, err := exec.Command(bin, "write-assembly", upstream).CombinedOutput(); err != nil {
		t.Fatalf("OCCT assembly author: %v %s", err, data)
	}
	raw, err := os.ReadFile(upstream)
	if err != nil {
		t.Fatal(err)
	}
	_, solids := checkPlanarVertexContactIGES(t, raw)
	var out bytes.Buffer
	if _, err := WritePlanarVertexContactAssembly(context.Background(), &out, solids, .001, PlanarVertexContactAssemblyOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(native, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{upstream, native} {
		data, err := exec.Command(bin, "read", file).CombinedOutput()
		if err != nil {
			t.Fatalf("pinned OCCT IGES read: %v %s", err, data)
		}
		var m struct {
			Solids, Shells, Faces int
			Volume, Area          float64
			Centroid              [3]float64
		}
		i := bytes.LastIndexByte(data, '{')
		if i < 0 || json.Unmarshal(data[i:], &m) != nil || m.Solids != 3 || m.Shells != 3 || m.Faces != 18 ||
			math.Abs(m.Volume-3) > 1e-9 || math.Abs(m.Area-18) > 1e-9 ||
			math.Abs(m.Centroid[0]-13./6) > 1e-9 || math.Abs(m.Centroid[1]-13./6) > 1e-9 || math.Abs(m.Centroid[2]-13./6) > 1e-9 {
			t.Fatalf("pinned OCCT assembly measures: %s", data)
		}
	}
}

func TestPlanarVertexContactAssemblyIGESCavity(t *testing.T) {
	raw, err := os.ReadFile("testdata/occt-planar-material-void.igs")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m := checkPlanarIGESVoids(t, raw, 1, 448)
	cube, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	v := cube.Vertices()
	for i, p := range v {
		v[i] = geom.Vec3{X: 2 + (p.X+p.Y+p.Z)/2, Y: 2 + (p.X+2*p.Y+p.Z)/2, Z: 2 + (p.X+p.Y+2*p.Z)/2}
	}
	insert, err := brep.NewSolid(v, cube.Edges(), cube.Faces())
	if err != nil {
		t.Fatal(err)
	}
	solids := []brep.Solid{m.Solid, insert}
	o := PlanarVertexContactAssemblyOptions{}
	var out bytes.Buffer
	if _, err := WritePlanarVertexContactAssembly(ctx, &out, solids, .001, o); !errors.Is(err, collision.ErrInvalid) || out.Len() != 0 {
		t.Fatalf("void shell was admitted without explicit opt-in: %v", err)
	}
	o.Proof.AllowCavities = true
	proof, err := WritePlanarVertexContactAssembly(ctx, &out, solids, .001, o)
	if err != nil || proof.SignedVolume.RatString() != "3585/8" || len(proof.Contacts) != 1 {
		t.Fatalf("exact IGES cavity contact: %+v %v", proof, err)
	}
	if m, err := ReadPlanarVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), PlanarVertexContactAssemblyOptions{}); err == nil || m.Bodies != nil {
		t.Fatalf("reader admitted cavity assembly without opt-in: %v", err)
	}
	reopened, err := ReadPlanarVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), o)
	if err != nil || len(reopened.Bodies) != 2 || len(reopened.Bodies[0].SourceShells) != 2 || !reopened.Bodies[0].SourceShells[1].Void {
		t.Fatalf("reopen lost original cavity ownership: %+v %v", reopened, err)
	}
	p := reopened.Proof.Contacts[0]
	a, b := reopened.Bodies[0], reopened.Bodies[1]
	want := geom.Vec3{X: 2, Y: 2, Z: 2}
	if a.Solid.Vertices()[p.A.Vertex] != want || b.Solid.Vertices()[p.B.Vertex] != want ||
		a.SourceVertexIndices[p.A.Vertex] == b.SourceVertexIndices[p.B.Vertex] {
		t.Fatalf("cavity vertex contact was welded or moved: %+v", p)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_PLANAR_SHELLS_IGES"); bin != "" {
		path := filepath.Join(t.TempDir(), "void-contact.igs")
		if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.Command(bin, "read", path).CombinedOutput()
		if err != nil {
			t.Fatalf("pinned OCCT cavity contact: %v %s", err, data)
		}
		var have struct {
			Solids, Shells, Faces int
			Volume, Area          float64
			Centroid              [3]float64
		}
		i := bytes.LastIndexByte(data, '{')
		if i < 0 || json.Unmarshal(data[i:], &have) != nil || have.Solids != 2 || have.Shells != 3 || have.Faces != 18 ||
			math.Abs(have.Volume-448.125) > 1e-9 || math.Abs(have.Area-(480+math.Sqrt2+math.Sqrt(11)/2)) > 1e-9 ||
			math.Abs(have.Centroid[0]-57355./14340) > 1e-9 ||
			math.Abs(have.Centroid[1]-14339./3585) > 1e-9 || math.Abs(have.Centroid[2]-14339./3585) > 1e-9 {
			t.Fatalf("pinned OCCT cavity contact measures: %s", data)
		}
	}
}
