package disk

import "strconv"

// size: 1.2T,960G
func ToMiB(size string) int {
	multiplier := map[string]int{
		"M": 1,
		"G": 1024,
		"T": 1024 * 1024,
		"P": 1024 * 1024 * 1024,
	}
	val, _ := strconv.Atoi(size[:len(size)-1])
	unit := size[len(size)-1:]
	return val * multiplier[unit]
}

func ToBytes(size string) int64 {
	multiplier := map[string]int64{
		"M": 1000 * 1000,
		"G": 1000 * 1000 * 1000,
		"T": 1000 * 1000 * 1000 * 1000,
		"P": 1000 * 1000 * 1000 * 1000 * 1000,
	}
	unit := size[len(size)-1:]
	val, _ := strconv.ParseInt(size[:len(size)-1], 10, 64)
	return val * multiplier[unit]
}
