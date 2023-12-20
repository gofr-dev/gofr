CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY,
    name varchar(50) ,
    age varchar(50)
) ENGINE = MergeTree ORDER BY id;
