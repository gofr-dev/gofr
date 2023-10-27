package main

import (
	"bytes"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func TestMain(m *testing.M) {
	app := gofr.New()

	table := "person"
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: aws.String("S")},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: aws.String("HASH")},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{ReadCapacityUnits: aws.Int64(10), WriteCapacityUnits: aws.Int64(5)},
		TableName:             aws.String(table),
	}

	_, err := app.DynamoDB.CreateTable(input)
	if err != nil {
		app.Logger.Errorf("Failed creation of table %v, %v", table, err)
	}

	os.Exit(m.Run())
}

func TestIntegration(t *testing.T) {
	go main()
	time.Sleep(2 * time.Second)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"create success case", http.MethodPost, "person", http.StatusCreated,
			[]byte(`{"id":"10", "name":  "gofr", "email": "gofr@gofr.dev.com"}`)},
		{"get success case", http.MethodGet, "person/10", http.StatusOK, nil},
		{"update success case", http.MethodPut, "person/10", http.StatusOK,
			[]byte(`{"id":"10", "name":  "gofr1", "email": "gofrone@gofr.dev.com"}`)},
		{"delete success case", http.MethodDelete, "person/10", http.StatusNoContent, nil},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:9091/"+tc.endpoint, bytes.NewBuffer(tc.body))

		cl := http.Client{}

		resp, err := cl.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v\n%s", i, err, tc.desc)
			continue
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		_ = resp.Body.Close()
	}
}
