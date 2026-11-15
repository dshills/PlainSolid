package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func obliqueFilletStock(t *testing.T, shift geom.Vec3) (brep.Solid, int) {
	t.Helper()
	p := []geom.Vec3{shift, shift.Add(geom.Vec3{X: 4}), shift.Add(geom.Vec3{Y: 3})}
	source, e := brep.Prism(p, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	vertices := source.Vertices()
	for i, edge := range source.Edges() {
		a, b := vertices[edge.Start], vertices[edge.End]
		if a.X == p[1].X && b.X == p[1].X && a.Y == p[1].Y && b.Y == p[1].Y {
			return source, i
		}
	}
	t.Fatal("original triangular prism corner edge is absent")
	return brep.Solid{}, -1
}

func TestFilletOriginalConvexPrismEdge(t *testing.T) {
	ctx := context.Background()
	const radius = 5.0 / 16
	sweep := math.Atan2(3, -4)
	area := 6 - radius*radius*(3-sweep/2)
	for _, shift := range []geom.Vec3{{}, {X: 2, Y: -4, Z: -3}} {
		source, edge := obliqueFilletStock(t, shift)
		s, h, e := FilletConvexPrismEdge(ctx, source, edge, radius, 1e-9)
		if e != nil || h.SourceEdge != edge || len(h.SourceFaces) != 5 || len(h.FilletFaces) != 4 {
			t.Fatal("original caps, walls and one circular fillet", h, e)
		}
		for _, index := range h.FilletFaces {
			c, ok := s.faces[index].Surface.(geom.CylinderSurface)
			if !ok {
				t.Fatal("analytic circular transition", index)
			}
			p, e := c.Eval(0, 0)
			if e != nil || p != shift.Add(geom.Vec3{X: 54.0 / 16, Y: 5.0 / 16}) {
				t.Fatal("exact common center and radius across cylinder charts", p, e)
			}
		}
		topo := s.Topology()
		if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 {
			t.Fatal("closed filleted prism", topo)
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		props, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(props.Volume-5*area) > 1e-8 {
			t.Fatal("analytic independent cross-section integrated over full prism height", props, e)
		}
		for _, q := range []struct {
			edge int
			r    float64
			want error
		}{{-1, radius, geom.ErrInvalid}, {edge, 0, geom.ErrInvalid}, {edge, 2, geom.ErrDomain}, {edge, .25, geom.ErrUnrepresentable}} {
			if s, h, e := FilletConvexPrismEdge(ctx, source, q.edge, q.r, 1e-9); !errors.Is(e, q.want) || len(s.faces) != 0 || h.SourceFaces != nil {
				t.Fatal("invalid fillet must publish no topology", q, h, e)
			}
		}
	}
}

func TestFilletOriginalConvexPrismEdgeOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_OFFSET to the pinned OCCT comparator")
	}
	ctx := context.Background()
	stock, edge := obliqueFilletStock(t, geom.Vec3{})
	s, _, e := FilletConvexPrismEdge(ctx, stock, edge, 5.0/16, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	compareOriginalConvexFilletOCCT(t, bin, "triangle-fillet", 6, s)
}

func TestFilletOriginalConvexPrismAdjacentEdges(t *testing.T) {
	ctx := context.Background()
	stock, acute := obliqueFilletStock(t, geom.Vec3{})
	vertices := stock.Vertices()
	right := -1
	for i, uv := range stock.Edges() {
		a, b := vertices[uv.Start], vertices[uv.End]
		if a.X == 0 && a.Y == 0 && b.X == 0 && b.Y == 0 {
			right = i
			break
		}
	}
	if right < 0 {
		t.Fatal("independent right-angle vertical edge is absent")
	}
	requests := []ConvexPrismEdgeRadius{{acute, 5.0 / 16}, {right, .25}}
	s, history, e := FilletConvexPrismEdges(ctx, stock, requests, 1e-9)
	if e != nil || len(history.SourceFaces) != 5 || len(history.Fillets) != 2 {
		t.Fatal("separate original edge histories", history, e)
	}
	wantCharts := map[int]int{acute: 4, right: 1}
	for _, f := range history.Fillets {
		if len(f.Faces) != wantCharts[f.SourceEdge] || wantCharts[f.SourceEdge] == 0 {
			t.Fatal("generated cylinder charts belong to the correct source edge", history)
		}
		delete(wantCharts, f.SourceEdge)
	}
	if topo := s.Topology(); len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 {
		t.Fatal("closed prism with two adjacent fillets", topo)
	}
	var a, b bytes.Buffer
	if e := WriteJSON(&a, s); e != nil {
		t.Fatal(e)
	}
	otherOrder, _, e := FilletConvexPrismEdges(ctx, stock, []ConvexPrismEdgeRadius{requests[1], requests[0]}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if e := WriteJSON(&b, otherOrder); e != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("atomic requests are independent of their submission order", e)
	}
	if s, h, e := FilletConvexPrismEdges(ctx, stock, []ConvexPrismEdgeRadius{requests[0], requests[0]}, 1e-9); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 || h.Fillets != nil {
		t.Fatal("duplicate source edge is rejected atomically", h, e)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_OFFSET"); bin != "" {
		compareOriginalConvexFilletOCCT(t, bin, "triangle-fillet-both", 7, s)
	}
}

func compareOriginalConvexFilletOCCT(t *testing.T, bin, original string, faceCount int, s Solid) {
	t.Helper()
	ctx := context.Background()
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := exec.Command(bin, original, "0.3125").CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	var valid, solids, faces int
	var volume, area float64
	if n, e := fmt.Sscan(string(raw), &valid, &solids, &faces, &volume, &area); e != nil || n != 5 || valid != 1 || solids != 1 || faces != faceCount ||
		math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
		t.Fatal("independent OCCT cylindrical fillet measures", p, string(raw), e)
	}
}
