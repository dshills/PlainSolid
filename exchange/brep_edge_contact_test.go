package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestEdgeContactPairNativeJSON(t *testing.T) {
	ctx := context.Background()
	var parts [2]brep.Solid
	for i, p := range []geom.Vec3{{}, {X: 1, Y: 1}} {
		s, err := brep.Box(p, p.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		if err != nil {
			t.Fatal(err)
		}
		parts[i] = s
	}
	o := EdgeContactPairJSONOptions{}
	var out, rewritten bytes.Buffer
	if err := WriteEdgeContactPairJSON(ctx, &out, parts, o); err != nil {
		t.Fatal(err)
	}
	reloaded, proof, err := ReadEdgeContactPairJSON(ctx, bytes.NewReader(out.Bytes()), o)
	if err != nil || proof.SignedVolume.RatString() != "2" {
		t.Fatalf("native reload lost exact original edge: %+v %v", proof, err)
	}
	for i := range parts {
		if !reflect.DeepEqual(parts[i].Vertices(), reloaded[i].Vertices()) ||
			!reflect.DeepEqual(parts[i].Edges(), reloaded[i].Edges()) ||
			!reflect.DeepEqual(parts[i].Faces(), reloaded[i].Faces()) {
			t.Fatalf("part %d original native graph was altered", i)
		}
	}
	if err := WriteEdgeContactPairJSON(ctx, &rewritten, reloaded, o); err != nil || !bytes.Equal(out.Bytes(), rewritten.Bytes()) {
		t.Fatalf("native receipt changed on strict re-export: %v", err)
	}
	for _, change := range []func(*edgeContactPairWitness){
		func(w *edgeContactPairWitness) { w.Edges[1] = (w.Edges[1] + 1) % 12 },
		func(w *edgeContactPairWitness) { w.Faces[0][0], w.Faces[0][1] = w.Faces[0][1], w.Faces[0][0] },
		func(w *edgeContactPairWitness) { w.Endpoints[0].Y += .5 },
		func(w *edgeContactPairWitness) { w.Reversed = !w.Reversed },
		func(w *edgeContactPairWitness) { w.Volume = "3" },
	} {
		var doc edgeContactPairDocument
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		change(doc.Witness)
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		got, p, err := ReadEdgeContactPairJSON(ctx, bytes.NewReader(b), o)
		if !errors.Is(err, collision.ErrInvalid) || p.SignedVolume != nil || len(got[0].Vertices()) != 0 {
			t.Fatalf("tampered native witness published original topology: %+v %v", p, err)
		}
	}
	limited := EdgeContactPairJSONOptions{MaxBytes: int64(out.Len() - 1)}
	if _, p, err := ReadEdgeContactPairJSON(ctx, bytes.NewReader(out.Bytes()), limited); !errors.Is(err, collision.ErrLimit) || p.SignedVolume != nil {
		t.Fatalf("bounded reader published a partial proof: %+v %v", p, err)
	}
	rewritten.Reset()
	if err := WriteEdgeContactPairJSON(ctx, &rewritten, parts, limited); !errors.Is(err, collision.ErrLimit) || rewritten.Len() != 0 {
		t.Fatalf("bounded writer published a partial receipt: %v", err)
	}
}

func TestEdgeIntervalContactPairNativeJSON(t *testing.T) {
	ctx := context.Background()
	a, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, err := brep.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 2, Y: 2, Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	edge := func(s brep.Solid) int {
		t.Helper()
		v := s.Vertices()
		for i, e := range s.Edges() {
			p, q := v[e.Start], v[e.End]
			if p.X == 1 && p.Y == 1 && q.X == 1 && q.Y == 1 {
				return i
			}
		}
		t.Fatal("no original directed shared-edge support")
		return -1
	}
	edges := [2]int{edge(a), edge(b)}
	parts := [2]brep.Solid{a, b}
	o := EdgeContactPairJSONOptions{}
	var out, again bytes.Buffer
	if err := WriteEdgeIntervalContactPairJSON(ctx, &out, parts, edges, o); err != nil {
		t.Fatal(err)
	}
	reloaded, proof, err := ReadEdgeIntervalContactPairJSON(ctx, bytes.NewReader(out.Bytes()), o)
	if err != nil || proof.EdgeA != edges[0] || proof.EdgeB != edges[1] ||
		proof.SignedVolume.RatString() != "6" || proof.Start.Z == proof.End.Z {
		t.Fatalf("native subinterval reproof: %+v %v", proof, err)
	}
	if err := WriteEdgeIntervalContactPairJSON(ctx, &again, reloaded, edges, o); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatalf("exact subinterval receipt changed on reload: %v", err)
	}
	if _, _, err := ReadEdgeContactPairJSON(ctx, bytes.NewReader(out.Bytes()), o); !errors.Is(err, collision.ErrInvalid) {
		t.Fatalf("whole-edge reader widened to a partial receipt: %v", err)
	}
	for _, change := range []func(*edgeContactPairWitness){
		func(w *edgeContactPairWitness) { w.Intervals[0][0] = "1/5" },
		func(w *edgeContactPairWitness) { w.Intervals[1] = append(w.Intervals[1], "1/2") },
	} {
		var doc edgeContactPairDocument
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		change(doc.Witness)
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if b, p, err := ReadEdgeIntervalContactPairJSON(ctx, bytes.NewReader(raw), o); !errors.Is(err, collision.ErrInvalid) ||
			len(b[0].Vertices()) != 0 || p.SignedVolume != nil {
			t.Fatalf("inconsistent persisted source interval published topology: %v", err)
		}
	}
}
