USE users;

CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY,
    name varchar(50) ,
    age varchar(50)
) ENGINE = MergeTree ORDER BY id;

INSERT INTO users (id, name, age) VALUES ('37387615-aead-4b28-9adc-78c1eb714ca2','stella','21');
