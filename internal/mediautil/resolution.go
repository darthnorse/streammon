package mediautil

import "strconv"

// HeightToResolution converts a video height to a resolution string.
// This is the canonical function for resolution string generation across all media adapters.
func HeightToResolution(height int) string {
	switch {
	case height >= 2160:
		return "4K"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 480:
		return "480p"
	case height > 0:
		return strconv.Itoa(height) + "p"
	default:
		return ""
	}
}
