# Connecting MySQL/MariaDB

Just like Redis GoFr also supports connection to SQL (MySQL, MariaDB, and Postgres) databases based on configuration variables.

## Setup

Users can run MySQL and create a database locally using the following Docker command:

```bash
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=root123 -e MYSQL_DATABASE=test_db -p 3306:3306 -d mysql:8.0.30
```

For MariaDB, you would run:

```bash
docker run --name gofr-mariadb -e MYSQL_ROOT_PASSWORD=root123 -e MYSQL_DATABASE=test_db -p 3306:3306 -d mariadb:latest
```


Access `test_db` database and create table customer with columns `id` and `name`. Change mysql to mariadb as needed: 

```bash
docker exec -it gofr-mysql mysql -uroot -proot123 test_db -e "CREATE TABLE customers (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255) NOT NULL);"
```

Now the database with table is ready, we can connect our GoFr server to MySQL/MariaDB. 

## Configuration & Usage

After adding MySQL/MariaDB configs `.env` will be updated to the following. Use ```DB_DIALECT=mysql``` for both MySQL and MariaDB.

```dotenv
# configs/.env
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379

DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306
DB_DIALECT=mysql
DB_CHARSET=

# DB_CHARSET: The character set for database connection (default: utf8).
# The `DB_CHARSET` defaults to utf8, but setting it to utf8mb4 is recommended if you need full Unicode support,
# including emojis and special characters.
```

Now in the following example, we'll store customer data using **POST** `/customer` and then use **GET** `/customer` to retrieve the same.
We will be storing the customer data with `id` and `name`.

After adding code to add and retrieve data from MySQL/MariaDB datastore, `main.go` will be updated to the following.

```go
package main

import (
	"errors"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr"
)

type Customer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	// initialise gofr object
	app := gofr.New()

	app.GET("/redis", func(ctx *gofr.Context) (any, error) {
		// Get the value using the Redis instance

		val, err := ctx.Redis.Get(ctx.Context, "test").Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			// If the key is not found, we are not considering this an error and returning ""
			return nil, err
		}

		return val, nil
	})

	app.POST("/customer/{name}", func(ctx *gofr.Context) (any, error) {
		name := ctx.PathParam("name")

		// Inserting a customer row in database using SQL
		_, err := ctx.SQL.ExecContext(ctx, "INSERT INTO customers (name) VALUES (?)", name)

		return nil, err
	})

	app.GET("/customer", func(ctx *gofr.Context) (any, error) {
		var customers []Customer

		// Getting the customer from the database using SQL
		rows, err := ctx.SQL.QueryContext(ctx, "SELECT * FROM customers")
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var customer Customer
			if err := rows.Scan(&customer.ID, &customer.Name); err != nil {
				return nil, err
			}

			customers = append(customers, customer)
		}

		// return the customer
		return customers, nil
	})

	app.Run()
}
```

To update the database with the customer data access use through this curl command through terminal

```bash
# here abc and xyz after /customer are the path parameters
curl --location --request POST 'http://localhost:9000/customer/abc'

curl --location --request POST 'http://localhost:9000/customer/xyz'
```
Now when we access {% new-tab-link title="http://localhost:9000/customer" href="http://localhost:9000/customer" /%} we should see the following output

```json
{
  "data": [
    {
      "id": 1,
      "name": "abc"
    },
    {
      "id": 2,
      "name": "xyz"
    }
  ]
}
```
