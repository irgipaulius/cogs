package grid

import "math"

// TileSpan returns the geographic size (degrees) of a climate tile at level.
func TileSpan(level int) float64 {
	switch level {
	case Level2Deg:
		return 360 // world single tile
	case LevelHalf:
		return 10
	case LevelEighth:
		return 2
	case LevelNative:
		return 1
	default:
		return 10
	}
}

// TileOrigin snaps south/west to the tile index for a level.
func TileOrigin(level int, south, west float64) (sy, sx int) {
	span := TileSpan(level)
	if level == Level2Deg {
		return -90, -180
	}
	sy = int(math.Floor(south/span)) * int(span)
	sx = int(math.Floor(west/span)) * int(span)
	if sx > 180 {
		sx = 180
	}
	if sx < -180 {
		sx = -180
	}
	return sy, sx
}

// TileBoundsForOrigin returns south, west, north, east for tile (sy,sx).
func TileBoundsForOrigin(level, sy, sx int) (south, west, north, east float64) {
	span := TileSpan(level)
	if level == Level2Deg {
		return -90, -180, 90, 180
	}
	south = float64(sy)
	west = float64(sx)
	north = south + span
	east = west + span
	if north > 90 {
		north = 90
	}
	if east > 180 {
		east = 180
	}
	return south, west, north, east
}

// CoarserFallback returns the next coarser level and snapped origin covering the same area.
func CoarserFallback(level, sy, sx int) (nextLevel, fsy, fsx int, found bool) {
	if level <= Level2Deg {
		return 0, 0, 0, false
	}
	next := level - 1
	span := TileSpan(next)
	fsy = int(math.Floor(float64(sy)/span)) * int(span)
	fsx = int(math.Floor(float64(sx)/span)) * int(span)
	if next == Level2Deg {
		return Level2Deg, -90, -180, true
	}
	return next, fsy, fsx, true
}
