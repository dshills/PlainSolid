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
