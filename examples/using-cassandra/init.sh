docker run --name gofr-cassandra -d -p 2003:9042 cassandra:4.1;
sleep 60;
docker exec -i gofr-cassandra cqlsh < ../../.github/setups/keyspace.cql