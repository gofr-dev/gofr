package model

type Employee struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Phone  int    `json:"contact_number"`
	DOB    string `json:"dob"`
}
