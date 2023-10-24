package models

// Customer is the type on which all the core layer's functionality is implemented on
type Customer struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
	City string `json:"city"`
}
