//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedStationBoreUnequalPartitions(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	outerStations := []WrappedLoftStation{{Z: -2, Radius: 2}, {Z: 0, Radius: 2.5}, {Z: 3, Radius: 1.5}}
	innerStations := []WrappedLoftStation{{Z: -2, Radius: .75}, {Z: -.5, Radius: 1}, {Z: 1, Radius: .5}, {Z: 3, Radius: .75}}
	a, _, e := LoftWrappedCircularStations(ctx, geom.Vec2{X: .125, Y: -.25}, outerStations, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := LoftWrappedCircularStations(ctx, geom.Vec2{X: .125, Y: -.25}, innerStations, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := CombineCoaxialWrappedStationLofts(ctx, a, b, parametric.Difference, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e := WriteJSON(&native, source); e != nil {
		t.Fatal(e)
	}
	source, e = ReadJSONContext(ctx, bytes.NewReader(native.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .1}}}
	m, e := source.TessellateWrappedStationBoreCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.StationVertices[0]) != 3 || len(m.StationVertices[1]) != 4 {
		t.Fatal("original station partitions changed")
	}
	if e := m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("contacts")
	}
	owners := map[[2]int]map[int]bool{}
	for i, tri := range m.Mesh.Triangles {
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if owners[key] == nil {
				owners[key] = map[int]bool{}
			}
			owners[key][m.OriginalFace[i]] = true
		}
	}
	if len(m.Mesh.Vertices)-len(owners)+len(m.Mesh.Triangles) != 0 {
		t.Fatal("genus")
	}
	wallIDs := [2][]int{m.Source.OuterWalls, m.Source.InnerWalls}
	stationData := [2][]WrappedLoftStation{outerStations, innerStations}
	for wall, rings := range m.StationVertices {
		for station, ring := range rings {
			want := map[int]bool{}
			if station == 0 {
				want[wallIDs[wall][0]] = true
				want[m.Source.Caps[0]] = true
			} else if station == len(rings)-1 {
				want[wallIDs[wall][station-1]] = true
				want[m.Source.Caps[1]] = true
			} else {
				want[wallIDs[wall][station-1]] = true
				want[wallIDs[wall][station]] = true
			}
			for i, a := range ring {
				b := ring[(i+1)%len(ring)]
				if a > b {
					a, b = b, a
				}
				got := owners[[2]int{a, b}]
				if len(got) != 2 {
					t.Fatal("station incidence")
				}
				for id := range want {
					if !got[id] {
						t.Fatal("original face lineage")
					}
				}
				if m.Mesh.Vertices[ring[i]].Z != stationData[wall][station].Z {
					t.Fatal("station moved")
				}
			}
		}
	}
	volume := func(stations []WrappedLoftStation) float64 {
		v := 0.0
		for i, a := range stations[:len(stations)-1] {
			b := stations[i+1]
			v += math.Pi * (b.Z - a.Z) * (a.Radius*a.Radius + a.Radius*b.Radius + b.Radius*b.Radius) / 3
		}
		return v
	}
	want := volume(outerStations) - volume(innerStations)
	got, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(got-want) > .02*want {
		t.Fatal(got, want)
	}
	t.Log(len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks, m.StationSamples, got, want)
	o.Band.Patch.MaxTriangleChecks = 1
	bad, e := source.TessellateWrappedStationBoreCertified(ctx, o)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial budget", e)
	}
}
