package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

// WriteWrappedCylinderBoreRimFillet writes the authenticated native boundary
// as one analytic type 186 solid with separate inward and outward cylinders,
// type 198 toroids and annular planes. The original circular coedge intervals
// are retained as shared type 100 model arcs.
func WriteWrappedCylinderBoreRimFillet(ctx context.Context, w io.Writer, s trimmed.Solid, unit float64, o geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	g, err := borefillet.Extract(ctx, s, o)
	if err != nil {
		return err
	}
	raw, err := wrappedCylinderBoreRimFilletIGES(g, unit)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if _, err := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, true, true, true, true, true); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

func wrappedCylinderBoreRimFilletIGES(g borefillet.Graph, unit float64) ([]byte, error) {
	var a directedIGESAuthor
	exact := func(q *big.Rat) float64 {
		x, err := geom.RepresentedExactScalar(q)
		if err != nil {
			a.err = err
		}
		return x
	}
	model := func(q [3]*big.Rat) []float64 {
		return []float64{exact(q[0]), exact(q[1]), exact(q[2])}
	}
	axis, ref := a.addReal(123, 0, 0, 0, 1), a.addReal(123, 0, 1, 0, 0)
	surfaces := make([]int, len(g.Faces))
	for i, f := range g.Faces {
		origin := a.addReal(116, 0, append(model(f.Origin), 0)...)
		switch f.Kind {
		case borefillet.Plane:
			surfaces[i] = a.addReal(190, 1, float64(origin), float64(axis), float64(ref))
		case borefillet.Cylinder:
			surfaces[i] = a.addReal(192, 1, float64(origin), float64(axis), exact(f.Major), float64(ref))
		case borefillet.Torus:
			surfaces[i] = a.addReal(198, 1, float64(origin), float64(axis), exact(f.Major), exact(f.Minor), float64(ref), 0, 0)
		}
	}
	points := []float64{float64(len(g.Vertices))}
	for _, v := range g.Vertices {
		points = append(points, model(v)...)
	}
	vertices := a.addReal(502, 1, points...)
	topology := []int{504, len(g.Edges)}
	transforms := map[int]int{}
	for _, e := range g.Edges {
		start, end := e.Start, e.End
		if !e.Forward {
			start, end = end, start
		}
		p, q := model(g.Vertices[start]), model(g.Vertices[end])
		var curve int
		switch c := e.Circle; {
		case c.Radius == nil:
			curve = a.addReal(110, 0, append(p, q...)...)
		case !c.Meridian:
			x := model(c.Center)
			curve = a.addReal(100, 0, x[2], x[0], x[1], p[0], p[1], q[0], q[1])
		default:
			x := model(c.Center)
			from, to := g.Vertices[start], g.Vertices[end]
			local := func(p [3]*big.Rat, i int) float64 { return exact(new(big.Rat).Sub(p[i], c.Center[i])) }
			curve = a.addReal(100, 0, 0, 0, 0, local(from, 0), local(from, 2), local(to, 0), local(to, 2))
			transforms[curve] = a.addReal(124, 0, 1, 0, 0, x[0], 0, 0, -1, x[1], 0, 1, 0, x[2])
		}
		topology = append(topology, curve, vertices, start+1, vertices, end+1)
	}
	edges := a.add(504, 1, topologyNumbers(topology...))
	shell := []int{514, len(g.Faces)}
	flag := func(positive bool) int {
		if positive {
			return 1
		}
		return 0
	}
	for i, f := range g.Faces {
		bounds := []int{510, surfaces[i], 1 + len(f.Holes), 1}
		for _, wire := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			loop := []int{508, len(wire)}
			for _, u := range wire {
				sense := flag(!u.Reversed == g.Edges[u.Edge].Forward)
				loop = append(loop, 0, edges, u.Edge+1, sense, 0)
			}
			bounds = append(bounds, a.add(508, 1, topologyNumbers(loop...)))
		}
		face := a.add(510, 1, topologyNumbers(bounds...))
		shell = append(shell, face, flag(!f.Reversed))
	}
	outer := a.add(514, 1, topologyNumbers(shell...))
	a.add(186, 0, topologyNumbers(186, outer, 1, 0))
	if a.err != nil {
		return nil, a.err
	}
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	raw, err := a.file(unit)
	if err != nil {
		return nil, err
	}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			id, err := field(string(line[73:]))
			if err != nil {
				return nil, err
			}
			if transform, ok := transforms[id]; ok {
				copy(line[48:56], fmt.Sprintf("%8d", transform))
			}
		}
	}
	return raw, nil
}
