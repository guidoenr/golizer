package render

var (
	defaultPalette = []rune("░▒▓█▓▒░░▒▓█▀▄█▌▐█▓▒░▀▄▀▄▀▄█▓▒░")
	boxPalette     = []rune("░▒▓█▚▞▛▜▙▟▀▄▌▐")
	linesPalette   = []rune("░▒│┃║█▐▌╱╲╳╋╬═▓▒")
	sparkPalette   = []rune("░▒▓•◦○◉●◐◑◒◓◔◕⦿⦾█▓▒")
	retroPalette   = []rune("░▒▓▀▄█▌▐▓▒░▀▄▀▄█")
	minimalPalette = []rune("░▒▓█▓▒░")
	blockPalette   = []rune("░▒▓█████▓▒░")
	bubblePalette  = []rune("░▒○◐◑●◉⬤▓█")
)

// Palette returns characters used for brightness mapping.
func Palette(name string) []rune {
	switch name {
	case "box":
		return boxPalette
	case "lines":
		return linesPalette
	case "spark":
		return sparkPalette
	case "retro":
		return retroPalette
	case "minimal":
		return minimalPalette
	case "block":
		return blockPalette
	case "bubble":
		return bubblePalette
	default:
		return defaultPalette
	}
}

// PaletteNames returns all palette identifiers.
func PaletteNames() []string {
	return []string{"default", "box", "lines", "spark", "retro", "minimal", "block", "bubble"}
}
