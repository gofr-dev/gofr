docker run --name gofr-yugabyte -p 2011:9042 -d yugabytedb/yugabyte:2.14.3.0-b33 bin/yugabyted start --daemon=false

sleep 30

docker exec -i gofr-yugabyte ycqlsh < ../../.github/setups/keyspace.ycql