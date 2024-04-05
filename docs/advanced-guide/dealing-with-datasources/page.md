# Dealing with SQL

GoFr simplifies the process of connecting to SQL databases where one needs to add respective configs in .env,
which allows to connect to different SQL dialects(MYSQL, PostgreSQL) without going into complexity of configuring connections.

With GoFr, connecting to different SQL databases is as straightforward as setting the DB_DIALECT environment variable to the respective dialect.
For instance, to connect with PostgreSQL, set `DB_DIALECT` to `postgres`. Similarly, To connect with MySQL, simply set `DB_DIALECT` to `mysql`.

## Usage
Add the following configs in .env file.

```bash
DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306

DB_DIALECT=postgres
```