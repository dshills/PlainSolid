package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestPlanarVertexContactAssemblySTEPNeverRoundsAContactCoordinate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := vertexContactSTEPFixture(t)
	for _, f := range []bool{false, true} {
		var out bytes.Buffer
		o := VertexContactSTEPOptions{FacetedWriter: f}
		if _, err := WriteVertexContactAssembly(ctx, &out, m, o); err != nil {
			t.Fatal(err)
		}
		old, changed := []byte("CARTESIAN_POINT('',(1.,1.,1.))"), []byte("CARTESIAN_POINT('',(1.00000000000000001,1.,1.))")
		if !bytes.Contains(out.Bytes(), old) {
			t.Fatal("emitted fixture lost its authored point")
		}
		mutant := bytes.Replace(out.Bytes(), old, changed, 1)
		if legacy, err := Read(bytes.NewReader(mutant), Limits{}); err != nil || len(legacy.Bodies) != 3 {
			t.Fatalf("legacy numeric transfer should still work: %v", err)
		}
		if r, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(mutant), o); !errors.Is(err, geom.ErrUnrepresentable) || len(r.Bodies) != 0 || p.SignedVolume != nil {
			t.Fatalf("rounded vertex admitted into the exact contact graph: %v", err)
		}
	}
	box, err := brep.Box(geom.Vec3{X: .1}, geom.Vec3{X: .5, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := WriteVertexContactAssembly(ctx, &out, Model{Bodies: []Body{{"decimal", box}}, LengthUnit: .001}, VertexContactSTEPOptions{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("0.1000000000000000055511151231257827021181583404541015625")) {
		t.Fatal("writer did not emit the exact finite decimal of the original represented point")
	}
	if got, _, err := ReadVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), VertexContactSTEPOptions{}); err != nil || len(got.Bodies) != 1 {
		t.Fatalf("exact expanded decimal did not reopen: %v", err)
	}
}

func TestPlanarVertexContactAssemblySTEPRejectsNonplanarOriginalWire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := vertexContactSTEPFixture(t)
	s := m.Bodies[0].Solid
	vertices := s.Vertices()
	for i, p := range vertices {
		if p == (geom.Vec3{X: 1, Y: 1, Z: 1}) {
			vertices[i].Z = math.Nextafter(1, 2)
		}
	}
	warped, err := brep.NewSolid(vertices, s.Edges(), s.Faces())
	if err != nil {
		t.Fatalf("fixture must be inside the ordinary BRep planarity tolerance: %v", err)
	}
	m.Bodies = []Body{{"tolerated planar fixture", warped}}
	var out bytes.Buffer
	if p, err := WriteVertexContactAssembly(ctx, &out, m, VertexContactSTEPOptions{}); !errors.Is(err, collision.ErrInvalid) || out.Len() != 0 || p.SignedVolume != nil {
		t.Fatalf("nonplanar original wire escaped exact STEP qualification: %v", err)
	}
}

func TestPlanarVertexContactAssemblySTEPObliqueExactSupports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := vertexContactSTEPFixture(t)
	for i := range m.Bodies {
		s := m.Bodies[i].Solid
		v := s.Vertices()
		for j, p := range v {
			// This affine shear has determinant one. Its edge ratios and
			// plane normals need not have rational unit-length components.
			v[j] = geom.Vec3{X: p.X + 2*p.Y, Y: 3*p.X + 7*p.Y, Z: 2*p.X - p.Y + p.Z}
		}
		changed, err := brep.NewSolid(v, s.Edges(), s.Faces())
		if err != nil {
			t.Fatal(err)
		}
		m.Bodies[i].Solid = changed
	}
	for _, faceted := range []bool{false, true} {
		var out bytes.Buffer
		o := VertexContactSTEPOptions{FacetedWriter: faceted}
		if _, err := WriteVertexContactAssembly(ctx, &out, m, o); err != nil {
			t.Fatalf("faceted=%v exact oblique writer: %v", faceted, err)
		}
		got, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), o)
		if err != nil || len(p.Contacts) != 1 || p.SignedVolume.Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatalf("faceted=%v oblique readback: %+v %v", faceted, p, err)
		}
		c := p.Contacts[0]
		if got.Bodies[c.A.Solid].Solid.Vertices()[c.A.Vertex] != (geom.Vec3{X: 3, Y: 10, Z: 2}) {
			t.Fatalf("faceted=%v original oblique contact lost: %+v", faceted, c)
		}
	}
}

func TestPlanarVertexContactAssemblySTEPRejectsNearDeclaredSupports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var out bytes.Buffer
	if _, err := WriteVertexContactAssembly(ctx, &out, vertexContactSTEPFixture(t), VertexContactSTEPOptions{}); err != nil {
		t.Fatal(err)
	}
	f, err := Parse(bytes.NewReader(out.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	findFirst := func(kind string) Record {
		t.Helper()
		for _, id := range f.Order {
			records := f.Entities[id].Records
			if len(records) == 1 && records[0].Type == kind {
				return records[0]
			}
		}
		t.Fatalf("missing %s", kind)
		return Record{}
	}
	line := findFirst("LINE")
	vector, err := simple(f, line.Args[2].Ref, "VECTOR", 3)
	if err != nil {
		t.Fatal(err)
	}
	plane := findFirst("PLANE")
	axis, err := simple(f, plane.Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, message string
		id            int64
	}{
		{"line", "declared line exactly", vector.Args[1].Ref},
		{"plane", "declared plane exactly", axis.Args[2].Ref},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := simple(f, c.id, "DIRECTION", 2)
			if err != nil {
				t.Fatal(err)
			}
			v := r.Args[1].Items
			old := fmt.Sprintf("#%d=DIRECTION('',(%s,%s,%s));", c.id, v[0].Text, v[1].Text, v[2].Text)
			component := [3]string{v[0].Text, v[1].Text, v[2].Text}
			for i, x := range component {
				if x == "0." {
					component[i] = "1.E-18"
					break
				}
			}
			changed := fmt.Sprintf("#%d=DIRECTION('',(%s,%s,%s));", c.id, component[0], component[1], component[2])
			mutant := bytes.Replace(out.Bytes(), []byte(old), []byte(changed), 1)
			if bytes.Equal(mutant, out.Bytes()) {
				t.Fatal("test did not modify the original support")
			}
			if _, err := Read(bytes.NewReader(mutant), Limits{}); err != nil {
				t.Fatalf("ordinary tolerant STEP reader rejected its control: %v", err)
			}
			m, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(mutant), VertexContactSTEPOptions{})
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(c.message)) || len(m.Bodies) != 0 || p.SignedVolume != nil {
				t.Fatalf("inconsistent source support yielded a compound report: %+v %v", p, err)
			}
		})
	}
}

func TestPlanarVertexContactAssemblySTEPExactPhysicalUnits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := vertexContactSTEPFixture(t)
	o := VertexContactSTEPOptions{}
	for _, unit := range []float64{.0254, .3048, math.Nextafter(.1, 1)} {
		m.LengthUnit = unit
		var out bytes.Buffer
		if _, err := WriteVertexContactAssembly(ctx, &out, m, o); err != nil {
			t.Fatalf("exact unit %.17g writer: %v", unit, err)
		}
		r, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(out.Bytes()), o)
		if err != nil || r.LengthUnit != unit || len(p.Contacts) != 1 {
			t.Fatalf("exact unit %.17g changed physical context or contact: %v", unit, err)
		}
		if unit != .0254 {
			continue
		}
		for _, c := range [][2]string{
			{"LENGTH_MEASURE(0.0254)", "LENGTH_MEASURE(0.0254000000000000001)"},
			{"DIMENSIONAL_EXPONENTS(1.,0.,0.,0.,0.,0.,0.)", "DIMENSIONAL_EXPONENTS(1.00000000000000001,0.,0.,0.,0.,0.,0.)"},
			{"GEOMETRIC_REPRESENTATION_CONTEXT(3)", "GEOMETRIC_REPRESENTATION_CONTEXT(3.00000000000000001)"},
		} {
			mutant := bytes.Replace(out.Bytes(), []byte(c[0]), []byte(c[1]), 1)
			if bytes.Equal(mutant, out.Bytes()) {
				t.Fatalf("exact STEP writer lost expected declaration %q", c[0])
			}
			if legacy, err := Read(bytes.NewReader(mutant), Limits{}); err != nil || legacy.LengthUnit != unit {
				t.Fatalf("mutation must be indistinguishable to ordinary numeric import: %v", err)
			}
			b, p, err := ReadVertexContactAssembly(ctx, bytes.NewReader(mutant), o)
			if err == nil || len(b.Bodies) != 0 || p.SignedVolume != nil {
				t.Fatalf("rounded physical context yielded an exact contact assembly: %q: %v", c[1], err)
			}
		}
	}
}
