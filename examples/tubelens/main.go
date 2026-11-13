// Command tubelens discovers an implicit section, seals both graph faces,
// and saves the original geometry needed to reconstruct the exact lens.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func run(ctx context.Context, w io.Writer) (trimmed.AlgebraicTubeLensVolumeBounds, error) {
	var empty trimmed.AlgebraicTubeLensVolumeBounds
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		return empty, e
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := range x {
		for j := range x {
			poles = append(poles, geom.Vec3{X: x[i], Y: x[j], Z: z[i] + z[j] - 25.0/256 + x[i]*x[j]/64})
		}
	}
	k = []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		return empty, e
	}
	q := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	var loop []trimmed.Use
	for i, x := range q {
		loop = append(loop, trimmed.Use{Edge: i, UVStart: x, UVEnd: q[(i+1)%4]})
	}
	p := [2]intersect.Patch{{Surface: a, Max: q[2]}, {Surface: b, Max: q[2]}}
	f := [2]trimmed.Face{{Surface: a, Outer: loop}, {Surface: b, Outer: loop}}
	o := trimmed.AlgebraicTrimmedPairSectionOptions{}
	cuts := []float64{.25, .3125, .6875, .75}
	o.Trace.GraphFallback.Enabled = true
	o.Trace.GraphFallback.UCuts, o.Trace.GraphFallback.VCuts = cuts, cuts
	o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := trimmed.SectionTrimmedSplinePair(ctx, p, f, nil, o)
	if e != nil {
		return empty, e
	}
	lens, e := section.SealAlgebraicTubeLens(ctx, trimmed.AlgebraicTubeLensOptions{})
	if e != nil {
		return empty, e
	}
	bounds, e := lens.BoundVolume(ctx, trimmed.AlgebraicTubeLensVolumeOptions{})
	if e != nil {
		return empty, e
	}
	return bounds, trimmed.WriteAlgebraicTubeLensJSON(w, lens)
}

func main() {
	output := flag.String("out", "tube-lens.json", "original graph and section recipe")
	flag.Parse()
	f, e := os.Create(*output)
	if e != nil {
		log.Fatal(e)
	}
	bounds, e := run(context.Background(), f)
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		log.Fatal(e)
	}
	fmt.Printf("Wrote %s; volume lies in [%s, %s]\n", *output, bounds.Volume.Lower.FloatString(9), bounds.Volume.Upper.FloatString(9))
}
