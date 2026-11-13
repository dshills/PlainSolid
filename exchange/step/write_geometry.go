package step

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

func writeStandalone(w io.Writer, unit float64, setKind, repKind string, geometry func(func(string) int64) []int64) error {
	if !validLengthUnit(unit) {
		return errors.New("step: invalid length unit")
	}
	b := bufio.NewWriter(w)
	var writeErr error
	line := func(format string, args ...any) {
		if writeErr == nil {
			_, writeErr = fmt.Fprintf(b, format+"\n", args...)
		}
	}
	next := int64(1)
	emit := func(s string) int64 { id := next; next++; line("#%d=%s;", id, s); return id }
	line("ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('PlainSolid geometry'),'2;1');\nFILE_NAME('geometry.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;")
	length := emitLengthUnit(emit, unit)
	angle := emit("(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))")
	solidAngle := emit("(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT())")
	ctx := emit(fmt.Sprintf("(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#%d,#%d,#%d)) REPRESENTATION_CONTEXT('',''))", length, angle, solidAngle))
	ids := geometry(emit)
	items := ids
	if setKind != "" {
		items = []int64{emit(setKind + "(''," + references(ids) + ")")}
	}
	rep := emit(fmt.Sprintf("%s('',%s,#%d)", repKind, references(items), ctx))
	app := emit("APPLICATION_CONTEXT('configuration controlled 3d designs of mechanical parts and assemblies')")
	emit(fmt.Sprintf("APPLICATION_PROTOCOL_DEFINITION('international standard','config_control_design',1994,#%d)", app))
	pc := emit(fmt.Sprintf("MECHANICAL_CONTEXT('',#%d,'mechanical')", app))
	dc := emit(fmt.Sprintf("DESIGN_CONTEXT('',#%d,'design')", app))
	product := emit(fmt.Sprintf("PRODUCT('1','geometry','',(#%d))", pc))
	formation := emit(fmt.Sprintf("PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('','',#%d,.NOT_KNOWN.)", product))
	def := emit(fmt.Sprintf("PRODUCT_DEFINITION('design','',#%d,#%d)", formation, dc))
	shape := emit(fmt.Sprintf("PRODUCT_DEFINITION_SHAPE('','',#%d)", def))
	emit(fmt.Sprintf("SHAPE_DEFINITION_REPRESENTATION(#%d,#%d)", shape, rep))
	line("ENDSEC;\nEND-ISO-10303-21;")
	if writeErr != nil {
		return writeErr
	}
	return b.Flush()
}
