package types

// PRResult represents the result of checking a PR
type PRResult struct {
	Number           int
	ChangelogDesc    string
	PRTitle          string
	Status           PRStatus
	Error            error
}

// PRStatus represents the status of a PR check
type PRStatus int

const (
	StatusGoodMatch PRStatus = iota
	StatusPotentialMismatch
	StatusNotFound
)

func (s PRStatus) String() string {
	switch s {
	case StatusGoodMatch:
		return "✅ Good match"
	case StatusPotentialMismatch:
		return "⚠️ Potential mismatch"
	case StatusNotFound:
		return "❌ Not found"
	default:
		return "Unknown status"
	}
}