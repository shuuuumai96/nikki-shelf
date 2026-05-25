package stats

type Response struct {
	TotalEntries  int            `json:"totalEntries"`
	CurrentStreak int            `json:"currentStreak"`
	MoodCounts    map[string]int `json:"moodCounts"`
	LastEntryDate string         `json:"lastEntryDate"`
}
