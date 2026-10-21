package store

import (
	"encoding/binary"
	"math"
)

const months = 12

type Cell struct {
	Level     int
	LatI      int
	LonI      int
	TempC     [months]int16
	PrecipMM  [months]uint16
	CloudPct  *[months]uint8
	SunnyDays *int
	YearCount int
}

func (c Cell) CenterLat(step float64) float64 {
	return 90 - (float64(c.LatI)+0.5)*step
}

func (c Cell) CenterLon(step float64) float64 {
	lon := -180 + (float64(c.LonI)+0.5)*step
	if lon >= 180 {
		lon -= 360
	}
	return lon
}

func PackTemp(t [months]int16) []byte {
	b := make([]byte, months*2)
	for i := range t {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(t[i]))
	}
	return b
}

func UnpackTemp(b []byte) [months]int16 {
	var t [months]int16
	for i := 0; i < months && i*2+1 < len(b); i++ {
		t[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return t
}

func PackPrecip(p [months]uint16) []byte {
	b := make([]byte, months*2)
	for i := range p {
		binary.LittleEndian.PutUint16(b[i*2:], p[i])
	}
	return b
}

func UnpackPrecip(b []byte) [months]uint16 {
	var p [months]uint16
	for i := 0; i < months && i*2+1 < len(b); i++ {
		p[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return p
}

func PackCloud(c [months]uint8) []byte {
	out := make([]byte, months)
	copy(out, c[:])
	return out
}

func UnpackCloud(b []byte) *[months]uint8 {
	if len(b) < months {
		return nil
	}
	var c [months]uint8
	copy(c[:], b[:months])
	return &c
}

// ExportRow matches the static Goldilocks JSON cell row.
func ExportRow(c Cell, step float64) []any {
	lat := math.Round(c.CenterLat(step)*1000) / 1000
	lon := math.Round(c.CenterLon(step)*1000) / 1000
	row := make([]any, 0, 39)
	row = append(row, lat, lon)
	for i := 0; i < months; i++ {
		row = append(row, int(c.TempC[i]))
	}
	for i := 0; i < months; i++ {
		row = append(row, int(c.PrecipMM[i]))
	}
	for i := 0; i < months; i++ {
		if c.CloudPct == nil {
			row = append(row, -1)
		} else {
			row = append(row, int(c.CloudPct[i]))
		}
	}
	if c.SunnyDays == nil {
		row = append(row, -1)
	} else {
		row = append(row, *c.SunnyDays)
	}
	return row
}
