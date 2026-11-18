package exchange_test

import (
	"context"
	"math"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func countTwoCirclesExchangeControls(t *testing.T, b trimmed.ExactBezierBoundary) int {
	t.Helper()
	n := 0
	for _, edge := range b.Edges {
		n += len(edge.Model.Control)
	}
	for _, face := range b.Faces {
		support, err := face.BSplineSupport()
		if err != nil {
			t.Fatal(err)
		}
		n += len(support.Data().Poles)
		for _, loop := range face.Loops {
			for _, use := range loop {
				n += len(use.UV.Control)
			}
		}
	}
	return n
}

func checkTwoCirclesExchangeReadback(t *testing.T, ctx context.Context, sources []trimmed.Solid, want []float64, st, ig []byte) {
	t.Helper()
	for _, f := range []struct {
		reader, format string
		data           []byte
	}{
		{os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), "step", st},
		{os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), "igs", ig},
	} {
		if f.reader == "" {
			continue
		}
		report := readPinnedExactComponents(t, ctx, f.format, f.data, f.reader)
		if report.Solids != len(sources) || len(report.Components) != len(sources) {
			t.Fatalf("%s changed the number of separate solid bodies: %+v", f.format, report)
		}
		unmatched := slices.Clone(report.Components)
		vertices := 0
		for i, body := range sources {
			topo := body.Topology()
			vertices += topo.Vertices
			index := slices.IndexFunc(unmatched, func(part pinnedExactComponentReport) bool {
				return matchTwoCirclesExchangeVertices(body.Vertices(), part.Vertices)
			})
			if index < 0 {
				t.Fatalf("%s changed the original vertices of component %d", f.format, i)
			}
			part := unmatched[index]
			unmatched = slices.Delete(unmatched, index, index+1)
			tolerance := max(1e-13, math.Abs(want[i])*1e-9)
			if part.Shells != 1 || part.Faces != topo.Faces || part.Edges != topo.Edges ||
				!(part.Volume > 0) || !(math.Abs(part.Volume-want[i]) <= tolerance) {
				t.Fatalf("%s changed component %d topology or signed volume: %+v", f.format, i, part)
			}
			checkTwoCirclesExchangeIncidence(t, f.format, body, part)
			t.Logf("%s component %d has %d faces, %d edges, %d vertices and signed volume %.16g", f.format, i, part.Faces, part.Edges, len(part.Vertices), part.Volume)
		}
		if report.IndexedVertices != vertices {
			t.Fatalf("%s changed aggregate vertex identity", f.format)
		}
	}
}

func matchTwoCirclesExchangeVertices(native []geom.Vec3, neutral [][3]float64) bool {
	if len(native) != len(neutral) {
		return false
	}
	expected := make(map[[3]float64]bool, len(native))
	for _, p := range native {
		expected[[3]float64{p.X, p.Y, p.Z}] = true
	}
	for _, p := range neutral {
		if !expected[p] {
			return false
		}
		delete(expected, p)
	}
	return len(expected) == 0
}

func checkTwoCirclesExchangeIncidence(t *testing.T, format string, source trimmed.Solid, part pinnedExactComponentReport) {
	t.Helper()
	vertexID := make(map[[3]float64]int, len(part.Vertices))
	for i, v := range part.Vertices {
		vertexID[v] = i
	}
	points := source.Vertices()
	unmatched := make(map[[2]int]int)
	for _, e := range source.Edges() {
		a, b := points[e.Start], points[e.End]
		u, v := vertexID[[3]float64{a.X, a.Y, a.Z}], vertexID[[3]float64{b.X, b.Y, b.Z}]
		unmatched[[2]int{min(u, v), max(u, v)}]++
	}
	for _, e := range part.EdgeVertices {
		id := [2]int{min(e[0], e[1]), max(e[0], e[1])}
		if unmatched[id] < 1 {
			t.Fatalf("%s added or rewired an original edge %v", format, e)
		}
		unmatched[id]--
	}
	for _, n := range unmatched {
		if n != 0 {
			t.Fatalf("%s omitted an original edge", format)
		}
	}
}
