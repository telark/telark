package constants

type IDConfig struct {
	Prefix      string
	Pattern     string
	HexSegments []int
}

const pattern = "^[0-9a-f]{5}-[0-9a-f]{4}-[0-9a-f]{4}$"

var (
	UserIDConfig = IDConfig{
		Prefix:      "u-",
		Pattern:     "^u-" + pattern,
		HexSegments: []int{5, 4, 4},
	}
	GroupIDConfig = IDConfig{
		Prefix:      "ug-",
		Pattern:     "^ug-" + pattern,
		HexSegments: []int{5, 4, 4},
	}
	CategoryIDConfig = IDConfig{
		Prefix:      "cat-",
		Pattern:     "^cat-" + pattern,
		HexSegments: []int{5, 4, 4},
	}
	RoleIDConfig = IDConfig{
		Prefix:      "r-",
		Pattern:     "^r-" + pattern,
		HexSegments: []int{5, 4, 4},
	}
)
