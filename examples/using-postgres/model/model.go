package model

import "github.com/google/uuid"

type Customer struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Phone int       `json:"phone"`
}
