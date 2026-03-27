package room

// Predefined rooms.
var All = []string{"lounge", "gallery", "suggestions"}

func IsValid(name string) bool {
	for _, r := range All {
		if r == name {
			return true
		}
	}
	return false
}
