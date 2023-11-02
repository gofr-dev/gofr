-- Drop the database if it exists
DROP DATABASE IF EXISTS customers;

-- Create the database
CREATE DATABASE customers;

-- Switch to the 'customers' database
USE customers;

-- Create the 'customers' table in the 'public' schema
CREATE TABLE public.customers (
                                  id UUID NOT NULL PRIMARY KEY,
                                  name VARCHAR(50),
                                  email VARCHAR(50),
                                  phone BIGINT
);

DROP TABLE IF EXISTS employees;
CREATE TABLE employees (id serial primary key, name varchar(50), phone varchar(20), email varchar(50), city varchar(50));
INSERT INTO employees (id, name, phone, email, city) VALUES (1,'Rohan','01222','rohan@zopsmart.com','Berlin');
INSERT INTO employees (id, name, phone, email, city) VALUES (2,'Aman','22234','aman@zopsmart.com','Florida');