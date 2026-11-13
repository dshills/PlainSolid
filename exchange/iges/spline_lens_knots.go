package iges

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

func igesLensKnots(k []float64) bool {
	if len(k) < 6 {
		return false
	}
	for i := 0; i < 3; i++ {
		if k[i] != 0 || k[len(k)-1-i] != 1 {
			return false
		}
	}
	for i := 3; i < len(k)-3; {
		v := k[i]
		if v <= 0 || v >= 1 {
			return false
		}
		j := i + 1
		for j < len(k)-3 && k[j] == v {
			j++
		}
		if j-i > 2 {
			return false
		}
		i = j
	}
	return true
}
func composeIGESLens(p *geom.SurfacePatch, uv *geom.ExactSplineCurve, refined bool) (*geom.ExactSplineCurve, geom.ParameterRectangle, error) {
	if !refined {
		x, e := p.ComposeParameterCurve(context.Background(), uv, curvedOvalOptions())
		return x, geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}, e
	}
	pieces, e := p.ComposeParameterCurvePieces(context.Background(), uv, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: curvedOvalOptions(), MaxPieces: 1, MaxRootSubdivisions: 512, MaxHullDepth: 8})
	if e != nil {
		return nil, geom.ParameterRectangle{}, e
	}
	if !pieces.Complete || !pieces.Continuous || len(pieces.Pieces) != 1 || pieces.Pieces[0].Start.Sign() != 0 || pieces.Pieces[0].End.Cmp(big.NewRat(1, 1)) != 0 {
		return nil, geom.ParameterRectangle{}, geom.ErrConvergence
	}
	return pieces.Pieces[0].Curve, pieces.Pieces[0].Cell, nil
}
