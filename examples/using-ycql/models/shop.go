package models

type Shop struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	State    string `json:"state"`
}
