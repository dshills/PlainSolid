package step

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// HollowConeBody is a narrowing annular frustum with positive radii at both ends.
type HollowConeBody struct {
	Name                                                                     string
	Frame                                                                    geom.Frame
	OuterBaseRadius, OuterTopRadius, InnerBaseRadius, InnerTopRadius, Height float64
}
type HollowConeModel struct {
	Bodies     []HollowConeBody
	LengthUnit float64
}

func WriteHollowCones(w io.Writer, m HollowConeModel) error {
	if w == nil || len(m.Bodies) == 0 {
		return errors.New("step: missing hollow cone writer/model")
	}
	flat := ConeModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		if _, e := hollowConicalSolid(b.Frame, b.OuterBaseRadius, b.OuterTopRadius, b.InnerBaseRadius, b.InnerTopRadius, b.Height); e != nil {
			return e
		}
		flat.Bodies = append(flat.Bodies, ConeBody{b.Name, b.Frame, b.OuterBaseRadius, b.OuterTopRadius, b.Height}, ConeBody{"", b.Frame, b.InnerBaseRadius, b.InnerTopRadius, b.Height})
	}
	var raw bytes.Buffer
	if e := WriteCones(&raw, flat); e != nil {
		return e
	}
	return writeAnnularWalls(w, raw.Bytes(), m.LengthUnit, len(m.Bodies))
}

func hollowConicalSolid(f geom.Frame, ob, ot, ib, it, h float64) (parametric.Solid, error) {
	if !finiteNumber(ob) || !finiteNumber(ot) || !finiteNumber(ib) || !finiteNumber(it) || !finiteNumber(h) || h <= 0 || it <= 0 || ib <= it || ob <= ot || ot <= it || ob <= ib {
		return parametric.Solid{}, errors.New("step: invalid narrowing hollow cone dimensions")
	}
	return parametric.Revolve(f, []geom.Vec2{{X: ib}, {X: ob}, {X: ot, Y: h}, {X: it, Y: h}})
}
