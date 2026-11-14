package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func wrappedFanJSONFixture(t *testing.T, n int, flip bool) (*WrappedAlgebraicFanShell, []byte) {
	t.Helper()
	ctx := context.Background()
	pairs := wrappedFanFixture(t, n, flip)
	p, e := CertifyWrappedAlgebraicFanEmbedding(ctx, pairs, geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewWrappedAlgebraicFanShell(ctx, p, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicFanShellJSON(ctx, &out, s, WrappedAlgebraicFanJSONOptions{}); e != nil {
		t.Fatal(e)
	}
	return s, out.Bytes()
}
func TestWrappedAlgebraicFanJSON(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		n    int
		flip bool
	}{{3, false}, {4, false}, {4, true}} {
		_, raw := wrappedFanJSONFixture(t, tc.n, tc.flip)
		s, e := ReadWrappedAlgebraicFanShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicFanJSONOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Faces()) != tc.n || s.Vertices()[s.CenterVertex()] != (geom.Vec3{X: 2}) {
			t.Fatal("native fan")
		}
		owners, _ := s.VertexSources(s.CenterVertex())
		if len(owners) != tc.n {
			t.Fatal("circular owner link")
		}
		native := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
		r, e := native.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
		used := s.ExactOperations() + s.embedding.ExactOperations()
		for i, pair := range s.embedding.proof.pairs {
			used += pair.ExactOperations() + pair.sources[0].ExactOperations()
			shift, _ := s.embedding.PeriodShift(i)
			if shift[0].Int64() != int64(i) {
				t.Fatal("source lift")
			}
		}
		var out bytes.Buffer
		if e := WriteWrappedAlgebraicFanShellJSON(ctx, &out, s, WrappedAlgebraicFanJSONOptions{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
			t.Fatal("deterministic persisted proof", e)
		}
		if q, e := ReadWrappedAlgebraicFanShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicFanJSONOptions{MaxWork: used - 1}); q != nil || e == nil {
			t.Fatal("cumulative exhaustion")
		}
		if q, e := ReadWrappedAlgebraicChainShellJSON(ctx, bytes.NewReader(raw), WrappedAlgebraicChainJSONOptions{}); q != nil || e == nil {
			t.Fatal("fan accepted by strict chain schema")
		}
	}
}
func TestWrappedAlgebraicFanJSONRejects(t *testing.T) {
	ctx := context.Background()
	s, raw := wrappedFanJSONFixture(t, 4, false)
	mutations := map[string]func(*wrappedFanRecipe){
		"central model":       func(n *wrappedFanRecipe) { n.Witness.Vertices[n.Center].Point[0] = "3" },
		"central index":       func(n *wrappedFanRecipe) { n.Center = 1 },
		"source center":       func(n *wrappedFanRecipe) { n.Centers[2] = 1 },
		"missing sector":      func(n *wrappedFanRecipe) { v := &n.Witness.Vertices[n.Center]; v.Sources = v.Sources[:3] },
		"repeated sector":     func(n *wrappedFanRecipe) { v := &n.Witness.Vertices[n.Center]; v.Sources[3] = v.Sources[0] },
		"closing mate":        func(n *wrappedFanRecipe) { n.Mates[3].Edges[0][1] = 0 },
		"closing orientation": func(n *wrappedFanRecipe) { n.Mates[3].ReverseB = false },
		"closing lift":        func(n *wrappedFanRecipe) { n.PeriodShifts[3][0] = "4" },
		"lifted source": func(n *wrappedFanRecipe) {
			var f wrappedAlgebraicFaceRecipe
			json.Unmarshal(n.Faces[3], &f)
			for _, loop := range f.Loops {
				for _, edge := range loop {
					for _, row := range edge {
						u, _ := new(big.Rat).SetString(row[0])
						w, _ := new(big.Rat).SetString(row[2])
						row[0] = u.Add(u, w.Mul(w, big.NewRat(4, 1))).RatString()
					}
				}
			}
			n.Faces[3], _ = json.Marshal(f)
		},
		"source orientation": func(n *wrappedFanRecipe) {
			var f wrappedAlgebraicFaceRecipe
			json.Unmarshal(n.Faces[2], &f)
			f.Reversed = true
			n.Faces[2], _ = json.Marshal(f)
		},
		"edge owner": func(n *wrappedFanRecipe) { n.Witness.Edges[0].Sources[0][0] = 2 },
		"exterior":   func(n *wrappedFanRecipe) { n.Witness.Boundary[0][0] = 3 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var n wrappedFanRecipe
			if e := json.Unmarshal(raw, &n); e != nil {
				t.Fatal(e)
			}
			mutate(&n)
			if name == "lifted source" {
				if _, e := ReadWrappedAlgebraicQuadraticFaceJSON(ctx, bytes.NewReader(n.Faces[3]), WrappedQuadraticFaceJSONOptions{}); e != nil {
					t.Fatal("valid standalone shifted source", e)
				}
			}
			bad, e := json.Marshal(n)
			if e != nil {
				t.Fatal(e)
			}
			if q, e := ReadWrappedAlgebraicFanShellJSON(ctx, bytes.NewReader(bad), WrappedAlgebraicFanJSONOptions{}); q != nil || e == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), []byte(" {}")...), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"proven":true`), 1)} {
		if q, e := ReadWrappedAlgebraicFanShellJSON(ctx, bytes.NewReader(bad), WrappedAlgebraicFanJSONOptions{}); q != nil || e == nil {
			t.Fatal("malformed")
		}
	}
	for _, opts := range []WrappedAlgebraicFanJSONOptions{{MaxWork: 1}, {MaxBytes: int64(len(raw) - 1)}} {
		if q, e := ReadWrappedAlgebraicFanShellJSON(ctx, bytes.NewReader(raw), opts); q != nil || e == nil {
			t.Fatal("read budget")
		}
		var out bytes.Buffer
		if e := WriteWrappedAlgebraicFanShellJSON(ctx, &out, s, opts); e == nil || out.Len() != 0 {
			t.Fatal("atomic publication")
		}
	}
	bad := *s
	graph := *s.graph
	graph.edges = s.Edges()
	curve := graph.edges[0].Curve.(wrappedAlgebraicTrimCurve)
	curve.b.X += .125
	graph.edges[0].Curve = curve
	bad.graph = &graph
	var out bytes.Buffer
	if e := WriteWrappedAlgebraicFanShellJSON(ctx, &out, &bad, WrappedAlgebraicFanJSONOptions{}); e == nil || out.Len() != 0 {
		t.Fatal("stale actual evaluator")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := ReadWrappedAlgebraicFanShellJSON(canceled, bytes.NewReader(raw), WrappedAlgebraicFanJSONOptions{}); q != nil || e == nil {
		t.Fatal("cancel")
	}
}
