package grid

import "math"

// Resolution levels (finer = higher number).
const (
	Level2Deg    = 0 // 2° overview
	LevelHalf    = 1 // 0.5°
	LevelEighth  = 2 // 0.125° (~14 km)
	LevelNative  = 3 // 1/24° (~4.6 km)
)

func Step(level int) float64 {
	switch level {
	case Level2Deg:
		return 2.0
	case LevelHalf:
		return 0.5
	case LevelEighth:
		return 0.125
	case LevelNative:
		return 1.0 / 24.0
	default:
		return 0.125
	}
}

func CenterLat(level, latI int) float64 {
	return 90 - (float64(latI)+0.5)*Step(level)
}

func CenterLon(level, lonI int) float64 {
	lon := -180 + (float64(lonI)+0.5)*Step(level)
	if lon >= 180 {
		lon -= 360
	}
	return lon
}

func LatIToIndex(level int, lat float64) int {
	if lat > 90 {
		lat = 90 - 1e-9
	}
	if lat < -90 {
		lat = -90 + 1e-9
	}
	return int(math.Floor((90-lat)/Step(level) - 0.5))
}

func LonIToIndex(level int, lon float64) int {
	if lon >= 180 {
		lon = 180 - 1e-9
	}
	if lon <= -180 {
		lon = -180 + 1e-9
	}
	return int(math.Floor((lon + 180) / Step(level) - 0.5))
}

func LatRange(level int, south, north float64) (minI, maxI int) {
	minI = LatIToIndex(level, north)
	maxI = LatIToIndex(level, south)
	if minI > maxI {
		minI, maxI = maxI, minI
	}
	return minI, maxI
}

func LonRange(level int, west, east float64) (minI, maxI int) {
	if east < west {
		// dateline wrap: query full range
		return 0, int(math.Ceil(360/Step(level))) - 1
	}
	minI = LonIToIndex(level, west)
	maxI = LonIToIndex(level, east)
	if minI > maxI {
		minI, maxI = maxI, minI
	}
	return minI, maxI
}

func LevelForZoom(z int) int {
	switch {
	case z <= 3:
		return Level2Deg
	case z <= 6:
		return LevelHalf
	case z <= 9:
		return LevelEighth
	default:
		return LevelNative
	}
}

// TileBounds returns north, west, south, east for a Web Mercator tile.
func TileBounds(z, x, y int) (north, west, south, east float64) {
	n := math.Pow(2, float64(z))
	west = float64(x)/n*360 - 180
	east = float64(x+1)/n*360 - 180
	northRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	southRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y+1)/n)))
	north = northRad * 180 / math.Pi
	south = southRad * 180 / math.Pi
	return north, west, south, east
}
