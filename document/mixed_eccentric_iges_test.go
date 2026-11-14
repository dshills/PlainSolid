package document

import (
	"bytes"
	"context"
	"math"
	"os"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/exchange/iges"
	"reflect"
	"strconv"
	"testing"
)

// This exercises the existing generic native asset/label schema. It does not
// invent an IGES reimport adapter or claim that labels are geometric proofs.
func TestMixedEccentricIGESNativeAssets(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("../exchange/iges/testdata/mixed-eccentric-material.igs")
	if e != nil {
		t.Fatal(e)
	}
	source, e := iges.ReadMixedTopologySolids(ctx, bytes.NewReader(raw), iges.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	state := State{LengthUnit: source.LengthUnit, Planar: map[string]brep.Solid{}, Parametric: map[string]parametric.Solid{}, Labels: map[string]Label{}}
	for _, body := range source.Bodies {
		key := "iges-" + strconv.Itoa(body.SourceSolidID)
		attrs := map[string]Attribute{"iges.source.solid": {Kind: "integer", Integer: int64(body.SourceSolidID)}, "iges.family": {Kind: "string", Text: string(body.Family)}}
		switch body.Family {
		case iges.MixedTopologyTetrahedron:
			state.Planar[key] = body.Tetrahedron.Solid
		case iges.MixedTopologyEccentricBlind:
			b := body.EccentricBlind
			state.Parametric[key] = b.Native.Solids[0]
			attrs["iges.source.floor"] = Attribute{Kind: "integer", Integer: int64(body.LocalToSource[b.SourceFaceIDs[4]])}
			for i, id := range b.Native.CutterCapFaces[0] {
				attrs["native.floor."+strconv.Itoa(i)] = Attribute{Kind: "integer", Integer: int64(id)}
			}
		case iges.MixedTopologyEccentricThrough:
			state.Parametric[key] = body.EccentricThrough.Native.Solids[0]
		default:
			t.Fatal("unexpected family")
		}
		state.Labels[key] = Label{Target: AssetLabel, TargetID: key, Name: string(body.Family), Attributes: attrs}
	}
	d, e := New(state, 0)
	if e != nil {
		t.Fatal(e)
	}
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e = Read(bytes.NewReader(saved.Bytes()), 4<<20, 0)
	if e != nil {
		t.Fatal(e)
	}
	got, _ := d.Snapshot()
	if len(got.Planar) != 1 || len(got.Parametric) != 2 || got.LengthUnit != .001 || !reflect.DeepEqual(got.Labels, state.Labels) {
		t.Fatal("lost mixed assets/provenance")
	}
	var again bytes.Buffer
	if e = d.Write(&again); e != nil || !bytes.Equal(saved.Bytes(), again.Bytes()) {
		t.Fatal("native instability", e)
	}
	volume, area := 0., 0.
	moment := [3]float64{}
	for _, s := range got.Planar {
		p, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		volume += p.Volume
		area += p.SurfaceArea
		moment[0] += p.Volume * p.Centroid.X
		moment[1] += p.Volume * p.Centroid.Y
		moment[2] += p.Volume * p.Centroid.Z
	}
	for key, s := range got.Parametric {
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		volume += p.Volume
		area += p.Area
		moment[0] += p.Volume * p.Centroid.X
		moment[1] += p.Volume * p.Centroid.Y
		moment[2] += p.Volume * p.Centroid.Z
		if label := got.Labels[key]; label.Name == string(iges.MixedTopologyEccentricBlind) {
			for name, a := range label.Attributes {
				if len(name) < 13 || name[:13] != "native.floor." {
					continue
				}
				f := s.Faces()[int(a.Integer)]
				p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
				if e != nil || math.Abs(p.Z-2) > 1e-12 {
					t.Fatal("floor ancestry lost", p, e)
				}
			}
		}
	}
	v := 4./3 + 66*math.Pi
	if math.Abs(volume-v) > 1e-8 || math.Abs(area-(6+2*math.Sqrt(3)+94*math.Pi)) > 1e-8 {
		t.Fatal(volume, area)
	}
	for i, want := range []float64{2./3 - 26*math.Pi, 2./3 - 3*math.Pi, 2./3 + 130*math.Pi} {
		if math.Abs(moment[i]-want) > 1e-8 {
			t.Fatal("mixed first moment", moment)
		}
	}
}
