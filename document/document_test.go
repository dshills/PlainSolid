package document

import (
	"bytes"
	"errors"
	"io"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sync"
	"testing"
)

func initial(t *testing.T) State {
	t.Helper()
	m, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return State{LengthUnit: .001, Assets: map[string]mesh.Mesh{"part": m}, Nodes: map[string]Node{"root": {Transform: geom.Translation(geom.Vec3{X: 10})}, "part1": {Parent: "root", Asset: "part", Transform: geom.Translation(geom.Vec3{Y: 2})}}}
}
func TestAssembly(t *testing.T) {
	s := initial(t)
	tr, e := s.WorldTransform("part1")
	if e != nil || tr.Apply(geom.Vec3{}) != (geom.Vec3{X: 10, Y: 2}) {
		t.Fatal(tr, e)
	}
	m, e := s.Flatten()
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() != 1 {
		t.Fatal(e)
	}
	b, _ := m.Bounds()
	if b.Min != (geom.Vec3{X: 10, Y: 2}) || b.Max != (geom.Vec3{X: 11, Y: 3, Z: 1}) {
		t.Fatal(b)
	}
	s.Nodes["part2"] = Node{Asset: "part", Transform: geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1})}
	m, e = s.Flatten()
	if e != nil || math.Abs(m.SignedVolume()-2) > 1e-12 {
		t.Fatal(m.SignedVolume(), e)
	}
}
func TestTransactions(t *testing.T) {
	s := initial(t)
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	s.Assets["part"].Vertices[0].X = 999
	a, b := d.Begin(), d.Begin()
	a.State.Nodes["root"] = Node{Name: "edited", Transform: geom.Identity()}
	if e = a.Commit(); e != nil {
		t.Fatal(e)
	}
	if e = b.Commit(); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	a.State.Assets["part"].Vertices[0].X = 77
	snap, r := d.Snapshot()
	if r != 1 || snap.Assets["part"].Vertices[0].X != 0 {
		t.Fatal("alias")
	}
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	snap, _ = d.Snapshot()
	if snap.Nodes["root"].Name != "" {
		t.Fatal("undo")
	}
	if e = d.Redo(); e != nil {
		t.Fatal(e)
	}
	snap, _ = d.Snapshot()
	if snap.Nodes["root"].Name != "edited" {
		t.Fatal("redo")
	}
	if e = a.Commit(); e == nil {
		t.Fatal("double commit")
	}
	tx := d.Begin()
	tx.Rollback()
	if e = tx.Commit(); e == nil {
		t.Fatal("rollback")
	}
	if e = d.Redo(); !errors.Is(e, ErrHistory) {
		t.Fatal(e)
	}
}
func TestHistoryLimit(t *testing.T) {
	d, _ := New(initial(t), 1)
	for i := 0; i < 3; i++ {
		tx := d.Begin()
		tx.State.LengthUnit = float64(i + 1)
		if e := tx.Commit(); e != nil {
			t.Fatal(e)
		}
	}
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	if e := d.Undo(); !errors.Is(e, ErrHistory) {
		t.Fatal(e)
	}
	tx := d.Begin()
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	if e := d.Redo(); !errors.Is(e, ErrHistory) {
		t.Fatal("redo survives branch")
	}
}
func TestValidation(t *testing.T) {
	for _, change := range []func(*State){func(s *State) { s.LengthUnit = 0 }, func(s *State) { n := s.Nodes["root"]; n.Parent = "part1"; s.Nodes["root"] = n }, func(s *State) { delete(s.Assets, "part") }, func(s *State) { n := s.Nodes["root"]; n.Parent = "missing"; s.Nodes["root"] = n }, func(s *State) { s.Nodes["root"] = Node{} }} {
		s := initial(t)
		change(&s)
		if e := s.Validate(); e == nil {
			t.Fatal("accepted")
		}
	}
}
func TestPersistence(t *testing.T) {
	d, _ := New(initial(t), 2)
	var b bytes.Buffer
	if e := d.Write(&b); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), b.Bytes()...)
	r, e := Read(&b, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := r.Snapshot()
	m, e := s.Flatten()
	if e != nil || m.SignedVolume() != 1 || s.LengthUnit != .001 {
		t.Fatal(e)
	}
	for _, data := range [][]byte{data[:10], append(data, []byte(" {}")...), []byte(`{"version":2}`), []byte(`{"version":1,"unknown":true}`)} {
		if _, e := Read(bytes.NewReader(data), 0, 1); e == nil {
			t.Fatal("accepted malformed")
		}
	}
	if _, e := Read(bytes.NewReader(data), 10, 1); e == nil {
		t.Fatal("limit")
	}
	if e := d.Write(shortWriter{}); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestConcurrentSnapshots(t *testing.T) {
	d, _ := New(initial(t), 10)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				tx := d.Begin()
				e := tx.Commit()
				if e != nil && !errors.Is(e, ErrConflict) {
					t.Error(e)
				}
				s, _ := d.Snapshot()
				if e = s.Validate(); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
}
func FuzzRead(f *testing.F) {
	f.Add([]byte(`{"version":1,"state":{"LengthUnit":1}}`))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = Read(bytes.NewReader(b), 4096, 1) })
}

func TestFlattenLimit(t *testing.T) {
	s := initial(t)
	if _, e := s.FlattenWithLimits(1, 100); e == nil {
		t.Fatal("vertex limit")
	}
	if _, e := s.FlattenWithLimits(100, 1); e == nil {
		t.Fatal("triangle limit")
	}
	if _, e := s.FlattenWithLimits(0, 1); e == nil {
		t.Fatal("invalid limit")
	}
}

func TestAccumulatedTransformUnderflow(t *testing.T) {
	tiny := geom.Scaling(geom.Vec3{X: 1e-200, Y: 1e-200, Z: 1e-200})
	s := State{LengthUnit: 1, Nodes: map[string]Node{"root": {Transform: tiny}, "child": {Parent: "root", Transform: tiny}}}
	if e := s.Validate(); e == nil {
		t.Fatal("accepted singular accumulated transform")
	}
	if _, e := s.WorldTransform("child"); e == nil {
		t.Fatal("silent transform underflow")
	}
}
