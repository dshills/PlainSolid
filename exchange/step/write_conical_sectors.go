package step

import (
	"errors"
	"io"
	"plainsolid/geom"
)

// ConicalSectorBody specifies a narrowing frustum sector with positive end radii.
type ConicalSectorBody struct {
	Name                                 string
	Frame                                geom.Frame
	BaseRadius, TopRadius, Height, Angle float64
}
type ConicalSectorModel struct {
	Bodies     []ConicalSectorBody
	LengthUnit float64
}

func WriteConicalSectors(w io.Writer, m ConicalSectorModel) error {
	for _, b := range m.Bodies {
		if !finiteNumber(b.BaseRadius) || !finiteNumber(b.TopRadius) || b.TopRadius <= 0 || b.BaseRadius <= b.TopRadius {
			return errors.New("step: conical sector requires narrowing positive radii")
		}
	}
	return writeSectorFrusta(w, m)
}
