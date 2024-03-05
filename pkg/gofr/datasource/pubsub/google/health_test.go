package google

import (
	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/testutil"
	"testing"
)

func TestGoogleClient_Health(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockClient := NewMockClient(ctrl)

	client := googleClient{
		Config: Config{},
		client: mockClient,
		logger: testutil.NewMockLogger(testutil.DEBUGLOG),
	}

	mockClient.EXPECT().Topics(gomock.Any()).Return(&pubsub.TopicIterator{})

	health := client.Health()

	assert.Equal(t, nil, health)
}
