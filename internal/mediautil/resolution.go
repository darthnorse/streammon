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

// HeightFromWidth returns the conventional logical-height tier for an encoded
// video width, in pixels. This is more stable than raw height for classifying
// content because aspect ratio doesn't shift the tier — a 1280-wide source is
// 720p whether it's encoded as 1280x720 (16:9) or 1280x540 (cinemascope).
// Returns 0 for widths below 720 (anything narrower is sub-SD and best left
// to the caller to handle).
func HeightFromWidth(width int) int {
	switch {
	case width >= 3840:
		return 2160
	case width >= 1920:
		return 1080
	case width >= 1280:
		return 720
	case width >= 720:
		return 480
	default:
		return 0
	}
}
