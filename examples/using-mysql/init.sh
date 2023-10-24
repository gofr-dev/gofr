#!/bin/bash

mkdir certificateFile
cd certificateFile

docker run --name gofr-ssl-mysql -e MYSQL_ROOT_PASSWORD=password -p 2001:3306 -d mysql:8.0.30

# Step 1: Generate Certificate Files
openssl genrsa -out ca-key.pem 4096
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 365 -out ca-cert.pem

openssl genrsa -out server-key.pem 4096
openssl req -new -key server-key.pem -out server-req.pem
openssl x509 -req -in server-req.pem -days 365 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem

openssl genrsa -out client-key.pem 4096
openssl req -new -key client-key.pem -out client-req.pem
openssl x509 -req -in client-req.pem -days 365 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem

# Step 2: Copy the client SSL certificate to the remote MySQL server's directory (Replace <source_path> and <container_id> accordingly)
docker cp ./ gofr-ssl-mysql:/usr/mysql-certificates

# Step 3: Update /etc/my.cnf file inside the container with the provided changes
# Step 3: Update /etc/my.cnf file inside the container with the provided changes
docker exec -i gofr-ssl-mysql bash -c 'cat > /etc/my.cnf' <<EOF
# For advice on how to change settings please see
# http://dev.mysql.com/doc/refman/8.0/en/server-configuration-defaults.html

[mysqld]
    max_connections=500
    require_secure_transport = ON

    max_allowed_packet=268435456
    open_files_limit=10000
    default-storage-engine=MyISAM
    innodb_file_per_table=1
    performance-schema=0

    ssl
    ssl-cipher=DHE-RSA-AES256-SHA
    ssl-ca=/usr/mysql-certificates/ca-cert.pem
    ssl-cert=/usr/mysql-certificates/server-cert.pem
    ssl-key=/usr/mysql-certificates/server-key.pem

skip-host-cache
skip-name-resolve
datadir=/var/lib/mysql
socket=/var/run/mysqld/mysqld.sock
secure-file-priv=/var/lib/mysql-files
user=mysql

pid-file=/var/run/mysqld/mysqld.pid
[client]
    ssl-mode=REQUIRED
    ssl-cert=/usr/mysql-certificates/client-cert.pem
    ssl-key=/usr/mysql-certificates/client-key.pem
socket=/var/run/mysqld/mysqld.sock

!includedir /etc/mysql/conf.d/
EOF

# Step 4: Update file permissions
docker exec gofr-ssl-mysql chown -R mysql:mysql /usr/mysql-certificates


# Step 5: Restart MySQL Docker container
docker restart gofr-ssl-mysql

sleep 30

#Step 6: To create customers database and employees table
docker exec -i gofr-ssl-mysql mysql -u root -ppassword < ../../../.github/setups/setupSSL.sql