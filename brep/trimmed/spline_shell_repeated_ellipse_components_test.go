package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestTangentEllipseExteriorComponents(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	tool := quarticRoofSolid(t, 1.0/16, 1.0/16)
	for a, pair := range [2][2]Solid{{stock, tool}, {tool, stock}} {
		options := SplineShellRepeatedEllipseOptions{Faces: [2]int{4, 1}}
		if a == 1 {
			options.Faces = [2]int{1, 4}
		}
		q, h, p, err := FuseExternallyTangentSplineShellEllipseComponents(ctx, pair[0], pair[1], options)
		shape := q.Topology()
		if err != nil || !p.Contact.Proven || p.SeparatedPairs != 35 || p.Enclosing != -1 ||
			h.Method != "repeated-ellipse-tangent-components" || shape.EulerCharacteristic != 4 ||
			len(shape.Shells) != 2 || shape.Faces != 12 || shape.Edges != 24 || shape.Vertices != 16 {
			t.Fatal("two original independent closed solids and their contact", shape, p, err)
		}
		for i, source := range pair {
			single, err := q.Shell(i)
			if err != nil || len(single.faces) != 6 || single.faces[options.Faces[i]].Surface != source.faces[options.Faces[i]].Surface {
				t.Fatal("component retains original chart identity", i, err)
			}
			sourceRows := [2][][]int{h.FacesA, h.FacesB}
			edgeRows := [2][][]int{h.EdgesA, h.EdgesB}
			if len(sourceRows[i][0]) != 1 || sourceRows[i][0][0] != 6*i || len(edgeRows[i][0]) != 1 || edgeRows[i][0][0] != 12*i {
				t.Fatal("component retains original face and edge ancestry", i, h)
			}
		}
		var data bytes.Buffer
		if err := WriteJSON(&data, q); err != nil {
			t.Fatal(err)
		}
		reload, err := ReadJSON(&data, JSONLimits{})
		if err != nil || len(reload.Topology().Shells) != 2 {
			t.Fatal("native component round trip", err)
		}
	}
	discovered, history, err := CombineSplineShells(ctx, stock, tool, parametric.Union, SplineShellCombineOptions{
		ClosedEllipseTangency: &geom.GraphPointContactOptions{},
	})
	if err != nil || len(discovered.Topology().Shells) != 2 || history.ClosedEllipse == nil ||
		!history.ClosedEllipse.Contact.Proven || history.ClosedEllipse.Faces != [2]int{4, 1} {
		t.Fatal("opt-in native Fuse discovers both contacting components", history, err)
	}
	enclosure, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, -2}, [3]float64{1.125, 1.125, 0}, 0)
	if q, h, p, err := FuseExternallyTangentSplineShellEllipseComponents(ctx, enclosure, tool, SplineShellRepeatedEllipseOptions{Faces: [2]int{5, 1}}); !errors.Is(err, geom.ErrConvergence) || len(q.faces) != 0 || h.Method != "" || p.Contact.Proven {
		t.Fatal("nested material cannot be retained as overlapping independent components", err)
	}
}

func TestOCCTTangentEllipseExteriorComponents(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_TANGENT_COMPONENTS")
	if oracle == "" {
		t.Skip("set PLAINSOLID_OCCT_TANGENT_COMPONENTS to the independent author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	path := filepath.Join(t.TempDir(), "ellipse-components.jsonl")
	if data, err := exec.CommandContext(ctx, oracle, path, "repeated-ellipse-tangent-components").CombinedOutput(); err != nil {
		t.Fatalf("independent OCCT exterior Fuse: %v %s", err, data)
	}
	r, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	d := json.NewDecoder(r)
	for _, expected := range []struct {
		name   string
		volume float64
	}{{"repeated-ellipse-stock-cut", 1}, {"repeated-ellipse-fused-components", 1 + 20227.0/20480}} {
		var p struct {
			Name   string
			Volume float64
		}
		if err := d.Decode(&p); err != nil || p.Name != expected.name || math.Abs(p.Volume-expected.volume) > 1e-10 {
			t.Fatal("independent material volume", expected, p, err)
		}
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatal("unexpected reference records", err)
	}
}
