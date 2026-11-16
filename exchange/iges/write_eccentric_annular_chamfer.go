package iges

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"strings"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/annularchamfer"
	"plainsolid/geom"
)

// WriteEccentricAnnularChamfer transfers the original conical wall shapes
// as lossless rational type 128 supports. The two original axes and the
// four chamfer slopes are preserved by the control nets. All original
// type 100 circular rims and type 110 generator trims are shared within
// a single type 186 solid.
func WriteEccentricAnnularChamfer(ctx context.Context, w io.Writer, s trimmed.Solid, unit float64, o geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	g, err := annularchamfer.Extract(ctx, s, o)
	if err != nil {
		return err
	}
	var a directedIGESAuthor
	exact := func(q *big.Rat) float64 {
		x, err := geom.RepresentedExactScalar(q)
		if err != nil {
			a.err = err
		}
		return x
	}
	model := func(p [3]*big.Rat) []float64 { return []float64{exact(p[0]), exact(p[1]), exact(p[2])} }
	axis, ref := a.addReal(123, 0, 0, 0, 1), a.addReal(123, 0, 1, 0, 0)
	supports := make([]int, len(g.Faces))
	for i, b := range g.Bands {
		if b.Patch == nil {
			origin := a.addReal(116, 0, append(model(g.Faces[i].Origin), 0)...)
			supports[i] = a.addReal(190, 1, float64(origin), float64(axis), float64(ref))
			continue
		}
		q, err := b.Patch.FundamentalPatch().ToBSplineSurface()
		if err != nil {
			return err
		}
		d := b.Patch.Domain()
		support, err := exactIGESSplineSupport(q.Data(), [4]float64{d.UMin, d.UMax, d.VMin, d.VMax}, false)
		if err != nil {
			return err
		}
		// The fundamental knot vector is clamped. The original edge at
		// UMin and UMax is shared and the graph already proves the seam.
		prop := strings.SplitN(support, ",", 7)
		prop[5] = "1"
		supports[i] = a.add(128, 0, strings.Join(prop, ","))
	}
	vertices := []float64{float64(len(g.Vertices))}
	for _, p := range g.Vertices {
		vertices = append(vertices, model(p)...)
	}
	vertexList := a.addReal(502, 1, vertices...)
	edges := []int{504, len(g.Edges)}
	for _, e := range g.Edges {
		i, j := e.Start, e.End
		if !e.Forward {
			i, j = j, i
		}
		p, q := model(g.Vertices[i]), model(g.Vertices[j])
		var curve int
		if e.Circle.Radius == nil {
			curve = a.addReal(110, 0, append(p, q...)...)
		} else {
			c := model(e.Circle.Center)
			curve = a.addReal(100, 0, c[2], c[0], c[1], p[0], p[1], q[0], q[1])
		}
		edges = append(edges, curve, vertexList, i+1, vertexList, j+1)
	}
	edgeList := a.add(504, 1, topologyNumbers(edges...))
	faceList := []int{514, len(g.Faces)}
	for i, f := range g.Faces {
		bounds := []int{510, supports[i], 1 + len(f.Holes), 1}
		for _, wire := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			loop := []int{508, len(wire)}
			for _, u := range wire {
				sense := igesSplineFaceOrientation(u.Reversed == g.Edges[u.Edge].Forward)
				loop = append(loop, 0, edgeList, u.Edge+1, sense, 0)
			}
			bounds = append(bounds, a.add(508, 1, topologyNumbers(loop...)))
		}
		id := a.add(510, 1, topologyNumbers(bounds...))
		faceList = append(faceList, id, igesSplineFaceOrientation(f.Reversed))
	}
	shell := a.add(514, 1, topologyNumbers(faceList...))
	a.add(186, 0, topologyNumbers(186, shell, 1, 0))
	if a.err != nil {
		return a.err
	}
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	raw, err := a.file(unit)
	if err != nil {
		return err
	}
	if _, err := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}
