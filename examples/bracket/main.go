// Command bracket models a rectangular mounting bracket and writes binary STL.
package main

import (
	"flag"
	"fmt"
	"os"
	"plainsolid/exchange"
	"plainsolid/geom"
	"plainsolid/solid"
)

func run(path string) error {
	base, err := solid.New(geom.Box{Min: geom.Vec3{}, Max: geom.Vec3{X: 40, Y: 30, Z: 4}}, geom.Box{Min: geom.Vec3{X: 0, Y: 0, Z: 4}, Max: geom.Vec3{X: 4, Y: 30, Z: 30}})
	if err != nil {
		return err
	}
	slot, err := solid.New(geom.Box{Min: geom.Vec3{X: 15, Y: 10, Z: -1}, Max: geom.Vec3{X: 25, Y: 20, Z: 5}})
	if err != nil {
		return err
	}
	part, err := base.Difference(slot)
	if err != nil {
		return err
	}
	m, err := part.Mesh(solid.Options{})
	if err != nil {
		return err
	}
	if err = m.ValidateSolid(); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	writeErr := exchange.WriteSTL(f, m)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Printf("wrote %s: volume=%g, triangles=%d\n", path, part.Volume(), len(m.Triangles))
	return nil
}
func main() {
	out := flag.String("out", "bracket.stl", "output STL path")
	flag.Parse()
	if e := run(*out); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
