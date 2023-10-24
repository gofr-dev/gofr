package person

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-dynamodb/models"
	"gofr.dev/examples/using-dynamodb/stores"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func TestMain(m *testing.M) {
	app := gofr.New()

	table := "person"
	deleteTableInput := &dynamodb.DeleteTableInput{TableName: aws.String(table)}

	_, err := app.DynamoDB.DeleteTable(deleteTableInput)
	if err != nil {
		app.Logger.Errorf("error in deleting table, %v", err)
	}

	createTableInput := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: aws.String("S")},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: aws.String("HASH")},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{ReadCapacityUnits: aws.Int64(10), WriteCapacityUnits: aws.Int64(5)},
		TableName:             aws.String(table),
	}

	_, err = app.DynamoDB.CreateTable(createTableInput)
	if err != nil {
		app.Logger.Errorf("Failed creation of table %v, %v", table, err)
	}

	os.Exit(m.Run())
}

func initializeTest(t *testing.T) (*gofr.Context, stores.Person) {
	app := gofr.New()

	// RefreshTables
	seeder := datastore.NewSeeder(&app.DataStore, "../../db")
	seeder.RefreshDynamoDB(t, "person")

	ctx := gofr.NewContext(nil, nil, app)

	store := New("person")

	return ctx, store
}

func TestGet(t *testing.T) {
	expResp := models.Person{ID: "1", Name: "Ponting", Email: "Ponting@gmail.com"}

	ctx, store := initializeTest(t)

	resp, err := store.Get(ctx, "1")
	if err != nil {
		t.Errorf("Failed\tExpected %v\nGot %v\n", nil, err)
	}

	assert.Equal(t, expResp, resp)
}

func TestGet_Error(t *testing.T) {
	app := gofr.New()

	ctx := gofr.NewContext(nil, nil, app)
	store := New("dummy")

	_, err := store.Get(ctx, "1")

	assert.IsType(t, errors.DB{}, err)
}

func TestCreate(t *testing.T) {
	input := models.Person{ID: "7", Name: "john", Email: "john@gmail.com"}

	ctx, store := initializeTest(t)

	err := store.Create(ctx, input)
	if err != nil {
		t.Errorf("Failed\tExpected %v\nGot %v\n", nil, err)
	}
}

func TestUpdate(t *testing.T) {
	input := models.Person{ID: "1", Name: "Ponting", Email: "Ponting.gates@gmail.com"}

	ctx, store := initializeTest(t)

	err := store.Update(ctx, input)
	if err != nil {
		t.Errorf("Failed\tExpected %v\nGot %v\n", nil, err)
	}
}

func TestDelete(t *testing.T) {
	ctx, store := initializeTest(t)

	err := store.Delete(ctx, "1")
	if err != nil {
		t.Errorf("Failed\tExpected %v\nGot %v\n", nil, err)
	}
}
