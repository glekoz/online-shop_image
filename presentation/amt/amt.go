package amt

import (
	"context"
	"encoding/json"
	"errors"

	amt "github.com/glekoz/online-shop_amt"
	"github.com/glekoz/online-shop_image/internal/models"
	"github.com/rabbitmq/amqp091-go"
)

type AppAPI interface {
	ProcessedSave(ctx context.Context, service, entityID, imageID, tmpImagePath string, isCover bool) error
}

type AMTHandler struct {
	App AppAPI
}

func NewAMTHandler(app AppAPI) *AMTHandler {
	return &AMTHandler{App: app}
}

func (a *AMTHandler) ProcessMessage(ctx context.Context, msg amqp091.Delivery) error {
	imgmsg := &models.ProcessImageMessage{}
	err := json.Unmarshal(msg.Body, imgmsg)
	if err != nil {
		return amt.NewErrNack("Invalid input")
	}
	err = a.App.ProcessedSave(ctx, imgmsg.Service, imgmsg.EntityID, imgmsg.ImageID, imgmsg.TmpImagePath, imgmsg.IsCover)
	if errors.Is(err, ctx.Err()) {
		return err
	}
	return amt.NewErrNack("Unprocessable entity")
}
