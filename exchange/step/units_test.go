package step

import (
	"bytes"
	"strings"
	"testing"
)

func TestConversionUnits(t *testing.T) {
	for _, u := range []float64{.0254, .3048, 1.23456789} {
		m := Model{LengthUnit: u, Bodies: []Body{{"part", advancedSolids(t)[0]}}}
		var b bytes.Buffer
		if e := WriteAdvanced(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := Read(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil || got.LengthUnit != u {
			t.Fatal(got.LengthUnit, e)
		}
		if got.Bodies[0].Solid.Vertices()[0] != m.Bodies[0].Solid.Vertices()[0] {
			t.Fatal("coordinates rescaled")
		}
		for _, change := range [][2]string{{"DIMENSIONAL_EXPONENTS(1.,0.", "DIMENSIONAL_EXPONENTS(0.,1."}, {"LENGTH_MEASURE(", "PLANE_ANGLE_MEASURE("}} {
			bad := strings.Replace(b.String(), change[0], change[1], 1)
			if bad == b.String() {
				t.Fatal(change)
			}
			if _, e := Read(strings.NewReader(bad), Limits{}); e == nil {
				t.Fatal(change)
			}
		}
	}
}
func TestUnitCyclesAndExactChain(t *testing.T) {
	raw := `ISO-10303-21;HEADER;FILE_DESCRIPTION(('units'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;DATA;
#1=(CONVERSION_BASED_UNIT('foot',#2) LENGTH_UNIT() NAMED_UNIT(#3));
#2=MEASURE_WITH_UNIT(LENGTH_MEASURE(12.),#4);
#3=DIMENSIONAL_EXPONENTS(1.,0.,0.,0.,0.,0.,0.);
#4=(CONVERSION_BASED_UNIT('inch',#5) LENGTH_UNIT() NAMED_UNIT(#3));
#5=MEASURE_WITH_UNIT(LENGTH_MEASURE(0.0254),#6);
#6=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT($,.METRE.));
ENDSEC;END-ISO-10303-21;`
	f, e := Parse(strings.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	u, e := lengthUnit(f, 1)
	if e != nil || u != .3048 {
		t.Fatal(u, e)
	}
	for _, bad := range []string{strings.Replace(raw, "0.0254),#6", "0.0254),#1", 1), strings.Replace(raw, "0.0254", "-1", 1), strings.Replace(raw, "0.0254", "1.E+308", 1)} {
		f, e := Parse(strings.NewReader(bad), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e := lengthUnit(f, 1); e == nil {
			t.Fatal("invalid chain")
		}
	}
}
