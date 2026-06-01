package ai

// Sanitize applies Presidio-style deterministic masks to trace text.
func Sanitize(input string) string {
	return NewScrubber().Scrub(input)
}

// ResetMaps clears per-session scrubbers (call after session ends).
func ResetMaps() {
	scrubMu.Lock()
	sessionScrub = make(map[string]*Scrubber)
	scrubMu.Unlock()
}
