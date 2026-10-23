package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"sort"

	"github.com/klauspost/compress/zstd"
)

var glkMagic = [4]byte{'G', 'L', 'K', '1'}

var zstdEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
var zstdDec, _ = zstd.NewReader(nil)

// EncodeGLK1 builds uncompressed GLK1 payload from cells at the given step.
func EncodeGLK1(step float64, sy, sx float64, cells []Cell) []byte {
	sorted := append([]Cell(nil), cells...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].LatI != sorted[j].LatI {
			return sorted[i].LatI < sorted[j].LatI
		}
		return sorted[i].LonI < sorted[j].LonI
	})
	var buf bytes.Buffer
	buf.Write(glkMagic[:])
	_ = binary.Write(&buf, binary.LittleEndian, float32(step))
	_ = binary.Write(&buf, binary.LittleEndian, float32(sy))
	_ = binary.Write(&buf, binary.LittleEndian, float32(sx))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(sorted)))
	for _, c := range sorted {
		lat := c.CenterLat(step)
		lon := c.CenterLon(step)
		_ = binary.Write(&buf, binary.LittleEndian, int32(math.Round(lat*1e6)))
		_ = binary.Write(&buf, binary.LittleEndian, int32(math.Round(lon*1e6)))
		for i := 0; i < months; i++ {
			_ = binary.Write(&buf, binary.LittleEndian, c.TempC[i])
		}
		for i := 0; i < months; i++ {
			_ = binary.Write(&buf, binary.LittleEndian, c.PrecipMM[i])
		}
		for i := 0; i < months; i++ {
			if c.CloudPct == nil {
				_ = buf.WriteByte(255)
			} else {
				_ = buf.WriteByte(c.CloudPct[i])
			}
		}
		if c.SunnyDays == nil {
			_ = binary.Write(&buf, binary.LittleEndian, uint16(0xFFFF))
		} else {
			_ = binary.Write(&buf, binary.LittleEndian, uint16(*c.SunnyDays))
		}
	}
	return buf.Bytes()
}

func CompressGLK1(raw []byte) []byte {
	return zstdEnc.EncodeAll(raw, nil)
}

func DecompressGLK1(compressed []byte) ([]byte, error) {
	return zstdDec.DecodeAll(compressed, nil)
}

// DecodedCell is one cell from GLK1 (for tests / optional server decode).
type DecodedCell struct {
	Lat, Lon       float64
	TempC          [months]int16
	PrecipMM       [months]uint16
	CloudPct       *[months]uint8
	SunnyDays      *int
	Step           float64
}

func DecodeGLK1(raw []byte) (step float64, sy, sx float64, cells []DecodedCell, err error) {
	if len(raw) < 16 || !bytes.Equal(raw[:4], glkMagic[:]) {
		return 0, 0, 0, nil, errors.New("bad glk1 magic")
	}
	r := bytes.NewReader(raw[4:])
	var fstep, fsy, fsx float32
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &fstep); err != nil {
		return 0, 0, 0, nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &fsy); err != nil {
		return 0, 0, 0, nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &fsx); err != nil {
		return 0, 0, 0, nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return 0, 0, 0, nil, err
	}
	step = float64(fstep)
	sy, sx = float64(fsy), float64(fsx)
	cells = make([]DecodedCell, 0, n)
	for i := uint32(0); i < n; i++ {
		var latU, lonU int32
		if err := binary.Read(r, binary.LittleEndian, &latU); err != nil {
			return 0, 0, 0, nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &lonU); err != nil {
			return 0, 0, 0, nil, err
		}
		var dc DecodedCell
		dc.Lat = float64(latU) / 1e6
		dc.Lon = float64(lonU) / 1e6
		dc.Step = step
		for m := 0; m < months; m++ {
			if err := binary.Read(r, binary.LittleEndian, &dc.TempC[m]); err != nil {
				return 0, 0, 0, nil, err
			}
		}
		for m := 0; m < months; m++ {
			if err := binary.Read(r, binary.LittleEndian, &dc.PrecipMM[m]); err != nil {
				return 0, 0, 0, nil, err
			}
		}
		var cloud [months]uint8
		hasCloud := false
		for m := 0; m < months; m++ {
			b, err := r.ReadByte()
			if err != nil {
				return 0, 0, 0, nil, err
			}
			if b != 255 {
				cloud[m] = b
				hasCloud = true
			}
		}
		if hasCloud {
			dc.CloudPct = &cloud
		}
		var sunnyU uint16
		if err := binary.Read(r, binary.LittleEndian, &sunnyU); err != nil {
			return 0, 0, 0, nil, err
		}
		if sunnyU != 0xFFFF {
			v := int(sunnyU)
			dc.SunnyDays = &v
		}
		cells = append(cells, dc)
	}
	return step, sy, sx, cells, nil
}
