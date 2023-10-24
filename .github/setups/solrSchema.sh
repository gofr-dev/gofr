curl 'http://localhost:8983/solr/admin/collections?action=CREATE&name=customer&numShards=1&replicationFactor=1'

curl -X POST -H 'Content-type:application/json' --data-binary '{
	"add-field": {
		"name": "id",
        	"type": "int",
         	"stored": "false",}
}' http://localhost:8983/solr/customer/schema

curl -X POST -H 'Content-type:application/json' --data-binary '{
	"add-field": {
		"name": "name",
		"type": "string",
		"stored": "true"}
}' http://localhost:8983/solr/customer/schema

curl -X POST -H 'Content-type:application/json' --data-binary '{
		"add-field":{
		   "name":"dateOfBirth",
		   "type":"string",
		"stored":true }
}' http://localhost:8983/solr/customer/schema




