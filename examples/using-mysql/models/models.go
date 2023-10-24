package models

type Employee struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone int    `json:"phone"`
	City  string `json:"city"`
}
