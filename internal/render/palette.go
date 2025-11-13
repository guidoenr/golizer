package render

var (
	defaultPalette = []rune("  .,:-;+=*%#@▓▒░█▚▞▛▜▙▟▘▝▗▖▞▚╱╲╳╋╬═║╔╗╚╝▤▥▧▨▩▦")
	boxPalette     = []rune(" ░▒▓█▚▞▛▜▙▟")
	linesPalette   = []rune(" `.-=+*/\\|╱╲╳╔╗╚╝═║╬")
	sparkPalette   = []rune("  ´`^\"~:;*+×•¤°oO@#█")
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
	default:
		return defaultPalette
	}
}

// PaletteNames returns all palette identifiers.
func PaletteNames() []string {
	return []string{"default", "box", "lines", "spark"}
}
