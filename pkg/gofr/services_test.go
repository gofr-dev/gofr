package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
)

func TestApp_AddOpenAI(t *testing.T) {
	t.Run("Adding OpenAI", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		c := container.NewContainer(config.NewMockConfig(nil))

		app := &App{
			container: c,
		}

		mock := container.NewMockOpenAIProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().UseTracer(gomock.Any())
		mock.EXPECT().InitMetrics()

		app.AddOpenAI(mock)

		assert.Equal(t, mock, app.container.OpenAI)
	})
}
