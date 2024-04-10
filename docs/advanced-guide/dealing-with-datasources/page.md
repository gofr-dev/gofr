# Dealing with SQL

GoFr simplifies the process of connecting to SQL databases where one needs to add respective configs in .env,
which allows to connect to different SQL dialects without going into complexity of configuring connections. 
Following are the currently supported SQL Dialects:
- mysql
- postgres
- sqlite

With GoFr, connecting to different SQL databases is as straightforward as setting the DB_DIALECT environment variable to the respective dialect.
For instance, to connect with PostgreSQL, set `DB_DIALECT` to `postgres`. Similarly, To connect with MySQL, simply set `DB_DIALECT` to `mysql`.

## Usage for MySQL and PostgreSQL
Add the following configs in .env file.

```dotenv
DB_DIALECT=postgres

DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306
```

## Usage for SQLite
To start using sqlite as the database, set the `DB_DIALECT` and `DB_HOST` configs as shown below:

```dotenv
DB_DIALECT=sqlite
DB_HOST=file:test.db # Data Source Name
```
>**Note:** 
> - For details on DSN string options, refer to the documentation [here](https://github.com/mattn/go-sqlite3?tab=readme-ov-file#connection-string). 


