package store

import "strings"

// Model represents the entity 'Customer'
type Model struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DateOfBirth string `json:"dateOfBirth"`
}

// Filter specifies the fields by which you can search a customer
type Filter struct {
	ID   string
	Name string
}

// GenSolrQuery makes a solr query based on the filters passed
func (f Filter) GenSolrQuery() string {
	query := ""

	if f.ID != "" {
		query += "id:" + f.ID + " AND "
	}

	if f.Name != "" {
		query += "name:" + f.Name + " AND "
	}

	query = strings.TrimSuffix(query, "AND ")

	return query
}
