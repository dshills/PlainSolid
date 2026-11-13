package occtcheck

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
	"testing"
)

func TestOCCTHollow(t *testing.T) {
	refs := runOracle(t, "feature")
	for _, open := range []int{-1, 1} {
		name := "hollow-cavity"
		thickness := []float64{.5, .5, .5, .5, .5, .5}
		if open == 1 {
			name = "hollow-tray"
			thickness[1] = 0
		}
		t.Run(name, func(t *testing.T) {
			s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
			if e != nil {
				t.Fatal(e)
			}
			r, e := draft.Hollow(context.Background(), s, thickness, draft.HollowOptions{OpenFace: open})
			if e != nil {
				t.Fatal(e)
			}
			m, e := r.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[name], m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
}

func TestOCCTHollowThrough(t *testing.T) {
	refs := runOracle(t, "feature")
	for _, name := range []string{"hollow-through-box", "hollow-through-sheared"} {
		t.Run(name, func(t *testing.T) {
			s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
			if err != nil {
				t.Fatal(err)
			}
			shear := name == "hollow-through-sheared"
			if shear {
				s, err = s.Transformed(geom.Affine{{1, 0, .5, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
				if err != nil {
					t.Fatal(err)
				}
			}
			r, err := draft.HollowThrough(context.Background(), s, []float64{0, 0, .25, .5, .75, 1},
				draft.ThroughHollowOptions{OpenFaces: [2]int{0, 1}, CoordinateThickness: shear})
			if err != nil {
				t.Fatal(err)
			}
			top := r.Solid.Topology()
			ref, ok := refs[name]
			if !ok || top.Shells != 1 || top.EulerCharacteristic != 0 || r.ExactVolume.RatString() != "60" {
				t.Fatal("expected a genus-one shell and exact volume 60", top, r.ExactVolume)
			}
			m, err := r.Solid.MassProperties()
			if err != nil {
				t.Fatal(err)
			}
			compare(t, ref, m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
}

func TestOCCTHollowThroughInclined(t *testing.T) {
	ref, ok := runOracle(t, "feature")["hollow-through-inclined"]
	if !ok {
		t.Fatal("missing independent inclined duct")
	}
	s, err := brep.Prism([]geom.Vec3{{}, {X: 4}, {X: 3, Z: 1}, {Z: 1}}, geom.Vec3{Y: 2})
	if err != nil {
		t.Fatal(err)
	}
	ends := [2]int{-1, -1}
	faces, edges, vertices := s.Faces(), s.Edges(), s.Vertices()
	thickness := make([]float64, len(faces))
	for i, f := range faces {
		left, right := true, true
		for _, u := range f.Boundary {
			p := vertices[edges[u.Edge].Start]
			left, right = left && p.X == 0, right && p.X+p.Z == 4
		}
		switch {
		case left:
			ends[0] = i
		case right:
			ends[1] = i
		default:
			thickness[i] = .25
		}
	}
	out, err := draft.HollowThrough(context.Background(), s, thickness, draft.ThroughHollowOptions{OpenFaces: ends})
	if err != nil {
		t.Fatal(err)
	}
	if top := out.Solid.Topology(); top.Shells != 1 || top.EulerCharacteristic != 0 || out.ExactVolume.RatString() != "35/8" {
		t.Fatal("expected one genus-one shell and exact volume 35/8", top, out.ExactVolume)
	}
	mass, err := out.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, ref, mass.SurfaceArea, mass.Volume, mass.Centroid)
}

func TestOCCTHollowThreePorts(t *testing.T) {
	ref := runOracle(t, "feature")["hollow-three-ports"]
	profile := []geom.Vec3{{X: 1}, {X: 3}, {X: 4, Y: 1}, {X: 4, Y: 4}, {X: 1, Y: 4}, {Y: 3}, {Y: 1}}
	s, err := brep.Prism(profile, geom.Vec3{Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	faces, edges, vertices := s.Faces(), s.Edges(), s.Vertices()
	thickness := make([]float64, len(faces))
	var ports []int
	for i, f := range faces {
		bottomLeft, bottomRight, topLeft := true, true, true
		for _, u := range f.Boundary {
			p := vertices[edges[u.Edge].Start]
			bottomLeft = bottomLeft && p.X+p.Y == 1
			bottomRight = bottomRight && p.X-p.Y == 3
			topLeft = topLeft && p.Y-p.X == 3
		}
		if bottomLeft || bottomRight || topLeft {
			ports = append(ports, i)
		} else {
			thickness[i] = .25
		}
	}
	if len(ports) != 3 {
		t.Fatal("expected three original port supports", ports)
	}
	r, err := draft.HollowPorts(context.Background(), s, thickness, draft.PortHollowOptions{OpenFaces: ports})
	if err != nil {
		t.Fatal(err)
	}
	if top := r.Solid.Topology(); top.Shells != 1 || top.EulerCharacteristic != -2 || r.ExactVolume.RatString() != "263/16" {
		t.Fatal("expected one genus-two shell and exact volume 263/16", top, r.ExactVolume)
	}
	m, err := r.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, ref, m.SurfaceArea, m.Volume, m.Centroid)
}
