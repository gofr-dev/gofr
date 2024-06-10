package datasource

type Cassandra interface {
	// Query executes the query and binds the result into dest parameter.
	// Returns error if any error occurs while binding the result.
	// Can be used to single as well as multiple rows.
	// Accepts pointer to struct or slice as dest parameter for single and multiple rows retrieval respectively.
	//
	// Example:
	//
	//	// Get multiple rows with only one column
	//	   ids := make([]int, 0)
	//	   err := c.Query(&ids, "SELECT id FROM users")
	//
	//	// Get a single object from database
	//	   type user struct {
	//	   	ID    int
	//	   	Name string
	//	   }
	//	   u := user{}
	//	   err := c.Query(&u, "SELECT * FROM users WHERE id=?", 1)
	//
	//	// Get array of objects from multiple rows
	//	   type user struct {
	//	   	ID    int
	//	   	Name string `db:"name"`
	//	   }
	//	   users := []user{}
	//	   err := c.Query(&users, "SELECT * FROM users")
	Query(dest interface{}, stmt string, values ...interface{}) error

	// Exec executes the query without returning any rows.
	// Return error if any error occurs while executing the query.
	// Can be used to execute UPDATE or INSERT.
	//
	// Example:
	//
	//	// Without values
	//	   err := c.Exec("INSERT INTO users VALUES(1, 'John Doe')")
	//
	//	// With Values
	//	   id := 1
	//	   name := "John Doe"
	//	   err := c.Exec("INSERT INTO users VALUES(?, ?)", id, name)
	Exec(stmt string, values ...interface{}) error

	// ExecCAS executes a lightweight transaction (i.e. an UPDATE or INSERT statement containing an IF clause).
	// If the transaction fails because the existing values did not match, the previous values will be stored in dest.
	// Returns true if the query is applied otherwise false.
	// Returns false and error if any error occur while executing the query.
	// Accepts only pointer to struct and built-in types as the dest parameter.
	//
	// Example:
	//
	//	type user struct {
	//		ID    int
	//		Name string
	//	}
	//	u := user{}
	//	applied, err := c.ExecCAS(&ids, "INSERT INTO users VALUES(1, 'John Doe') IF NOT EXISTS")
	ExecCAS(dest interface{}, stmt string, values ...interface{}) (bool, error)
}

type CassandraProvider interface {
	Cassandra

	// UseLogger sets the logger for the Cassandra client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the Cassandra client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
	Connect()
}
