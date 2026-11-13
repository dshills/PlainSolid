// Command boredplate builds an obliquely drilled plate and exports the
// editable document and its perspective hidden-line drawing.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"os"

	"plainsolid/brep/parametric"
	"plainsolid/document"
	"plainsolid/geom"
	"plainsolid/scene"
)

func main() {
	native := flag.String("native", "bored-plate.json", "editable native document")
	svg := flag.String("svg", "bored-plate.svg", "perspective drawing")
	flag.Parse()
	if *native == *svg {
		fmt.Fprintln(os.Stderr, "choose distinct output paths")
		os.Exit(1)
	}
	if e := run(context.Background(), *native, *svg); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

func run(ctx context.Context, native, svg string) error {
	s := document.State{
		LengthUnit: .001,
		Features: map[string]document.Feature{
			"blank": {Kind: document.BoxFeature, Max: geom.Vec3{X: 8, Y: 6, Z: 4}},
			"plate": {Kind: document.CylinderBoreFeature, Input: "blank", EntryRole: "z-min", ExitRole: "z-max",
				Center: geom.Vec3{X: 3, Y: 3}, Direction: geom.Vec3{X: .25, Z: 1}, Radius: .8, Tolerance: 1e-9},
		},
		Nodes: map[string]document.Node{"plate": {Name: "Bored plate", Asset: "plate", Transform: geom.Identity()}},
		Labels: map[string]document.Label{"plate.material": {Name: "Plate", Target: document.AssetLabel, TargetID: "plate",
			Attributes: map[string]document.Attribute{"material": {Kind: "string", Text: "Aluminum"}}}},
	}
	d, e := document.New(s, 8)
	if e != nil {
		return e
	}
	c, e := scene.Perspective(geom.Vec3{X: 12, Y: 12, Z: 15}, geom.Vec3{X: 4, Y: 3, Z: 2}, geom.Vec3{Z: 1}, math.Pi/3, 1.4, 1, 100)
	if e != nil {
		return e
	}
	lines, e := s.DrawVisibleEdges(ctx, c, document.DrawingOptions{
		Tessellation: parametric.TessellationOptions{Deflection: .06},
		Curves:       geom.CurveTessellationOptions{ChordTolerance: .03},
		Occlusion:    scene.HiddenLineOptions{DepthTolerance: .04, MaxTests: 5000000},
	})
	if e != nil {
		return e
	}
	if e = write(native, d.Write); e != nil {
		return e
	}
	return write(svg, func(w io.Writer) error { return scene.WriteSVG(w, lines, 840, 600) })
}

func write(path string, f func(io.Writer) error) error {
	w, e := os.Create(path)
	if e != nil {
		return e
	}
	err := f(w)
	closeErr := w.Close()
	if err != nil {
		return err
	}
	return closeErr
}
