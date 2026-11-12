package model

type Activity struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Duration    int     `json:"duration"` // dalam menit
	Date        string  `json:"date"`
	Description string  `json:"description,omitempty"`
}