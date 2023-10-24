package person

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"gofr.dev/examples/using-dynamodb/models"
	"gofr.dev/examples/using-dynamodb/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type store struct {
	table string
}

// New factory function for person store
func New(table string) stores.Person {
	return store{table: table}
}

func (s store) Create(ctx *gofr.Context, p models.Person) error {
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"id":    {S: aws.String(p.ID)},
			"name":  {S: aws.String(p.Name)},
			"email": {S: aws.String(p.Email)},
		},
		TableName: aws.String(s.table),
	}

	_, err := ctx.DynamoDB.PutItem(input)

	return err
}

func (s store) Get(ctx *gofr.Context, id string) (models.Person, error) {
	input := &dynamodb.GetItemInput{
		AttributesToGet: []*string{aws.String("id"), aws.String("name"), aws.String("email")},
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(id)},
		},
		TableName: aws.String(s.table),
	}

	var p models.Person

	out, err := ctx.DynamoDB.GetItem(input)
	if err != nil {
		return p, errors.DB{Err: err}
	}

	err = dynamodbattribute.UnmarshalMap(out.Item, &p)
	if err != nil {
		return p, errors.DB{Err: err}
	}

	return p, nil
}

func (s store) Update(ctx *gofr.Context, person models.Person) error {
	input := &dynamodb.UpdateItemInput{
		AttributeUpdates: map[string]*dynamodb.AttributeValueUpdate{
			"name":  {Value: &dynamodb.AttributeValue{S: aws.String(person.Name)}, Action: aws.String("PUT")},
			"email": {Value: &dynamodb.AttributeValue{S: aws.String(person.Email)}, Action: aws.String("PUT")},
		},
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(person.ID)},
		},
		TableName: aws.String(s.table),
	}

	_, err := ctx.DynamoDB.UpdateItem(input)

	return err
}

func (s store) Delete(ctx *gofr.Context, id string) error {
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(id)},
		},
		TableName: aws.String(s.table),
	}

	_, err := ctx.DynamoDB.DeleteItem(input)

	return err
}
