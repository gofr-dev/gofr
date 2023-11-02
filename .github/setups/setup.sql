DROP DATABASE IF EXISTS customers;
CREATE DATABASE customers;
connect customers
CREATE TABLE public.customers (
                                  id uuid NOT NULL primary key,
                                  name character varying(50),
                                  email varchar(50),
                                      phone bigint
);

DROP TABLE IF EXISTS employees;
CREATE TABLE employees (id serial primary key, name varchar(50), phone varchar(20), email varchar(50), city varchar(50));
INSERT INTO employees (id, name, phone, email, city) VALUES (1,'Rohan','01222','rohan@zopsmart.com','Berlin');
INSERT INTO employees (id, name, phone, email, city) VALUES (2,'Aman','22234','aman@zopsmart.com','Florida');