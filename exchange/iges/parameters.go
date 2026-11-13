package iges

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

type parameter struct {
	text        string
	stringValue bool
}

func parseParameters(s string, sep, end byte, budget *int) ([]parameter, error) {
	var values []parameter
	pos := 0
	for {
		for pos < len(s) && s[pos] == ' ' {
			pos++
		}
		if pos >= len(s) {
			return nil, errors.New("iges: missing parameter terminator")
		}
		start := pos
		for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
			pos++
		}
		v := parameter{}
		if pos > start && pos < len(s) && (s[pos] == 'H' || s[pos] == 'h') {
			n, e := strconv.Atoi(s[start:pos])
			if e != nil || n < 0 || n > len(s)-pos-1 {
				return nil, errors.New("iges: invalid Hollerith string length")
			}
			pos++
			v = parameter{text: s[pos : pos+n], stringValue: true}
			pos += n
			for pos < len(s) && s[pos] == ' ' {
				pos++
			}
			if pos == len(s) || (s[pos] != sep && s[pos] != end) {
				return nil, errors.New("iges: invalid Hollerith delimiter")
			}
		} else {
			pos = start
			for pos < len(s) && s[pos] != sep && s[pos] != end {
				pos++
			}
			v.text = strings.TrimSpace(s[start:pos])
		}
		*budget--
		if *budget < 0 {
			return nil, errors.New("iges: parameter limit exceeded")
		}
		values = append(values, v)
		if pos >= len(s) {
			return nil, errors.New("iges: missing parameter terminator")
		}
		delimiter := s[pos]
		pos++
		if delimiter == end {
			if strings.TrimSpace(s[pos:]) != "" {
				return nil, errors.New("iges: trailing parameter data")
			}
			return values, nil
		}
	}
}
func intValue(p parameter) (int, error) {
	if p.stringValue || p.text == "" {
		return 0, errors.New("iges: expected integer")
	}
	v, e := strconv.Atoi(p.text)
	if e != nil {
		return 0, errors.New("iges: invalid integer")
	}
	return v, nil
}
func realValue(p parameter) (float64, error) {
	if p.stringValue || p.text == "" {
		return 0, errors.New("iges: expected real")
	}
	s := strings.ReplaceAll(strings.ReplaceAll(p.text, "D", "E"), "d", "E")
	v, e := strconv.ParseFloat(s, 64)
	if !realLexeme(s) || e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("iges: invalid finite real")
	}
	return v, nil
}
func realLexeme(s string) bool {
	i := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i == len(s) {
		return true
	}
	if s[i] != 'E' && s[i] != 'e' {
		return false
	}
	i++
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > start && i == len(s)
}

func isDelimiter(b byte) bool {
	return b >= 33 && b <= 126 && !(b >= '0' && b <= '9') && !(b >= 'a' && b <= 'z') && !(b >= 'A' && b <= 'Z') && !strings.ContainsRune("+-.\"", rune(b))
}
func globalParameters(s string, budget *int) ([]parameter, byte, byte, error) {
	s = strings.TrimLeft(s, " ")
	sep, end := byte(','), byte(';')
	if len(s) == 0 {
		return nil, 0, 0, errors.New("iges: empty globals")
	}
	firstEnd := 1
	if s[0] != ',' {
		if len(s) < 3 || s[0] != '1' || (s[1] != 'H' && s[1] != 'h') {
			return nil, 0, 0, errors.New("iges: invalid global delimiter")
		}
		sep = s[2]
		firstEnd = 3
		for firstEnd < len(s) && s[firstEnd] == ' ' {
			firstEnd++
		}
		if !isDelimiter(sep) || firstEnd >= len(s) || s[firstEnd] != sep {
			return nil, 0, 0, errors.New("iges: invalid parameter separator")
		}
		firstEnd++
	}
	tail := strings.TrimLeft(s[firstEnd:], " ")
	if len(tail) == 0 {
		return nil, 0, 0, errors.New("iges: missing record delimiter")
	}
	if tail[0] != sep {
		if len(tail) < 3 || tail[0] != '1' || (tail[1] != 'H' && tail[1] != 'h') {
			return nil, 0, 0, errors.New("iges: invalid record delimiter")
		}
		end = tail[2]
		pos := 3
		for pos < len(tail) && tail[pos] == ' ' {
			pos++
		}
		if pos >= len(tail) || tail[pos] != sep {
			return nil, 0, 0, errors.New("iges: invalid record delimiter separator")
		}
	}
	if !isDelimiter(end) || end == sep {
		return nil, 0, 0, errors.New("iges: overlapping delimiters")
	}
	v, e := parseParameters(s, sep, end, budget)
	return v, sep, end, e
}

func noPointers(p []parameter) error {
	if len(p) > 2 {
		return errors.New("iges: unsupported trailing parameters")
	}
	for _, v := range p {
		if v.text == "" && !v.stringValue {
			continue
		}
		n, e := intValue(v)
		if e != nil || n != 0 {
			return errors.New("iges: associativity/property pointers unsupported")
		}
	}
	return nil
}

type unit struct {
	flag   int
	name   string
	meters float64
}

var units = []unit{{1, "IN", .0254}, {2, "MM", .001}, {4, "FT", .3048}, {5, "MI", 1609.344}, {6, "M", 1}, {7, "KM", 1000}, {8, "MIL", .0000254}, {9, "UM", .000001}, {10, "CM", .01}, {11, "UIN", .0000000254}}

func lengthUnit(v []parameter) (float64, error) {
	if len(v) < 24 || len(v) > 26 {
		return 0, errors.New("iges: expected 24 to 26 global parameters")
	}
	for _, i := range []int{2, 3, 4, 5, 11, 17, 20, 21, 24, 25} {
		if i < len(v) && v[i].text != "" && !v[i].stringValue {
			return 0, errors.New("iges: malformed global string")
		}
	}
	for _, i := range []int{6, 7, 8, 9, 10, 15, 22, 23} {
		if v[i].text != "" {
			n, e := intValue(v[i])
			if e != nil || n < 0 {
				return 0, errors.New("iges: malformed global integer")
			}
		}
	}
	for _, i := range []int{16, 18, 19} {
		if v[i].text != "" {
			x, e := realValue(v[i])
			if e != nil || x < 0 || (i == 18 && x == 0) {
				return 0, errors.New("iges: malformed global real")
			}
		}
	}
	scale, e := realValue(v[12])
	if e != nil || scale != 1 {
		return 0, errors.New("iges: only explicit model-space scale 1 supported")
	}
	flag, e := intValue(v[13])
	if e != nil {
		return 0, e
	}
	if !v[14].stringValue {
		return 0, errors.New("iges: explicit unit name required")
	}
	name := strings.ToUpper(v[14].text)
	for _, u := range units {
		if u.flag == flag {
			aliases := map[string]string{"INCH": "IN", "INCHES": "IN", "FEET": "FT", "MILES": "MI", "METERS": "M", "METRES": "M", "KILOMETERS": "KM", "MILS": "MIL", "MICRONS": "UM", "CENTIMETERS": "CM", "MICROINCH": "UIN"}
			if alias, ok := aliases[name]; ok {
				name = alias
			}
			if name != u.name {
				return 0, errors.New("iges: unit flag/name mismatch")
			}
			return u.meters, nil
		}
	}
	return 0, errors.New("iges: unsupported length unit")
}
