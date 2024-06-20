# Dealing with SQL

GoFr simplifies the process of connecting to SQL databases where one needs to add respective configs in .env,
which allows connecting to different SQL dialects(MySQL, PostgreSQL, SQLite) without going into complexity of configuring connections.

With GoFr, connecting to different SQL databases is as straightforward as setting the DB_DIALECT environment variable to the respective dialect.

## Usage for PostgreSQL and MySQL
To connect with PostgreSQL, set `DB_DIALECT` to `postgres`. Similarly, To connect with MySQL, simply set `DB_DIALECT` to `mysql`.

```dotenv
DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306

DB_DIALECT=postgres
```

## Usage for SQLite
To connect with PostgreSQL, set `DB_DIALECT` to `sqlite` and `DB_NAME` to the name of your DB File. If the DB file already exists then it will be used otherwise a new one will be created.

```dotenv
DB_NAME=test.db

DB_DIALECT=sqlite
```

## Setting Max open and Idle Connections

To set max open and idle connection for any MYSQL, PostgreSQL, sqlite. 
Add the following configs in `.env` file.

```dotenv
DB_MAX_IDLE_CONNECTION=5 // Default 2
DB_MAX_OPEN_CONNECTION=5 // Default unlimited
```