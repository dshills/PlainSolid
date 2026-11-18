package step

import (
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
)

const independentAssembly = `ISO-10303-21;
HEADER;FILE_DESCRIPTION(('independent assembly'),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;
DATA;
#1=APPLICATION_CONTEXT('assembly');#2=PRODUCT_CONTEXT('',#1,'mechanical');#3=PRODUCT_DEFINITION_CONTEXT('',#1,'design');
#4=PRODUCT('root','Machine','',(#2));#5=PRODUCT_DEFINITION_FORMATION('','',#4);#6=PRODUCT_DEFINITION('','',#5,#3);
#7=PRODUCT('part','Bracket','',(#2));#8=PRODUCT_DEFINITION_FORMATION('','',#7);#9=PRODUCT_DEFINITION('','',#8,#3);
#10=CARTESIAN_POINT('',(0.,0.,0.));#11=AXIS2_PLACEMENT_3D('',#10,$,$);
#12=CARTESIAN_POINT('',(10.,0.,0.));#13=AXIS2_PLACEMENT_3D('',#12,$,$);
#14=(LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.));
#15=(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#14)) REPRESENTATION_CONTEXT('',''));
#16=SHAPE_REPRESENTATION('',(#11,#13),#15);#17=SHAPE_REPRESENTATION('',(#11),#15);
#18=PRODUCT_DEFINITION_SHAPE('','',#6);#19=SHAPE_DEFINITION_REPRESENTATION(#18,#16);
#20=PRODUCT_DEFINITION_SHAPE('','',#9);#21=SHAPE_DEFINITION_REPRESENTATION(#20,#17);
#22=NEXT_ASSEMBLY_USAGE_OCCURRENCE('a','Placed bracket','',#6,#9,$);
#23=PRODUCT_DEFINITION_SHAPE('','',#22);
#24=ITEM_DEFINED_TRANSFORMATION('','',#11,#13);
#25=(REPRESENTATION_RELATIONSHIP('','',#17,#16) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#24) SHAPE_REPRESENTATION_RELATIONSHIP());
#26=CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#25,#23);
ENDSEC;END-ISO-10303-21;`

func TestIndependentAssembly(t *testing.T) {
	t.Parallel()
	a, e := ReadAssembly(strings.NewReader(independentAssembly), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 2 || len(a.Occurrences) != 1 || len(a.Roots) != 1 || a.Roots[0] != 6 || a.LengthUnit != .001 || a.Occurrences[0].Transform.Apply(geom.Vec3{X: 2}) != (geom.Vec3{X: 12}) {
		t.Fatal(a)
	}
	b, e := a.PlanarBodies(6, Limits{})
	if e != nil || len(b) != 0 {
		t.Fatal(b, e)
	}
	if _, e = Read(strings.NewReader(independentAssembly), Limits{}); e == nil {
		t.Fatal("shape reader flattened assembly")
	}
}
func TestAssemblyFailures(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{{"#6,#9,$", "#6,#6,$"}, {"#17,#16)", "#16,#17)"}, {"#11,#13);", "#13,#11);"}, {"ITEM_DEFINED_TRANSFORMATION", "CARTESIAN_TRANSFORMATION_OPERATOR_3D"}, {"CONTEXT_DEPENDENT_SHAPE_REPRESENTATION", "UNKNOWN"}, {"AUTOMOTIVE_DESIGN", "UNKNOWN"}} {
		bad := strings.Replace(independentAssembly, pair[0], pair[1], 1)
		if bad == independentAssembly {
			t.Fatal(pair)
		}
		if _, e := ReadAssembly(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(pair)
		}
	}
	for _, l := range []Limits{{MaxSolids: 1}, {MaxDepth: 1}, {MaxBytes: 10}} {
		if _, e := ReadAssembly(strings.NewReader(independentAssembly), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestOCCTAssemblyImport(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "assembly.step")
	stepOracle(t, path, "write-step-assembly")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadAssembly(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 2 || len(a.Occurrences) != 2 || len(a.Roots) != 1 {
		t.Fatal(a)
	}
	var child int64
	for _, p := range a.Products {
		if p.Name == "Bracket" {
			child = p.DefinitionID
		}
	}
	if child == 0 {
		t.Fatal("product name")
	}
	bodies, e := a.PlanarBodies(child, Limits{})
	if e != nil || len(bodies) != 1 {
		t.Fatal(bodies, e)
	}
	props, e := bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(props.Volume-6) > 1e-10 {
		t.Fatal(props, e)
	}
	for _, o := range a.Occurrences {
		if o.Child != child || o.Parent != a.Roots[0] {
			t.Fatal(o)
		}
		want := geom.Vec3{}
		switch o.Name {
		case "Left bracket":
			want = geom.Vec3{X: 11}
		case "Right bracket":
			want = geom.Vec3{Y: 21}
		default:
			t.Fatal(o.Name)
		}
		if o.Transform.Apply(geom.Vec3{X: 1}).Distance(want) > 1e-10 {
			t.Fatal(o)
		}
	}
}
func FuzzReadAssembly(f *testing.F) {
	f.Add(independentAssembly)
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadAssembly(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxSolids: 100, MaxEdgeUses: 100})
	})
}

func TestAssemblyCycleAndNonidentitySourceFrame(t *testing.T) {
	t.Parallel()
	cycle := `#27=NEXT_ASSEMBLY_USAGE_OCCURRENCE('b','cycle','',#9,#6,$);
#28=PRODUCT_DEFINITION_SHAPE('','',#27);
#29=ITEM_DEFINED_TRANSFORMATION('','',#11,#11);
#30=(REPRESENTATION_RELATIONSHIP('','',#16,#17) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#29) SHAPE_REPRESENTATION_RELATIONSHIP());
#31=CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#30,#28);`
	text := strings.Replace(independentAssembly, "ENDSEC;END-ISO-10303-21;", cycle+"ENDSEC;END-ISO-10303-21;", 1)
	if _, e := ReadAssembly(strings.NewReader(text), Limits{}); e == nil {
		t.Fatal("cycle")
	}
	text = strings.Replace(independentAssembly, "#17=SHAPE_REPRESENTATION('',(#11),#15)", "#17=SHAPE_REPRESENTATION('',(#28),#15)", 1)
	text = strings.Replace(text, "ITEM_DEFINED_TRANSFORMATION('','',#11,#13)", "ITEM_DEFINED_TRANSFORMATION('','',#28,#13)", 1)
	text = strings.Replace(text, "ENDSEC;END-ISO-10303-21;", "#27=CARTESIAN_POINT('',(2.,0.,0.));#28=AXIS2_PLACEMENT_3D('',#27,$,$);ENDSEC;END-ISO-10303-21;", 1)
	a, e := ReadAssembly(strings.NewReader(text), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Occurrences[0].Transform.Apply(geom.Vec3{X: 2}) != (geom.Vec3{X: 10}) {
		t.Fatal(a.Occurrences)
	}
}
