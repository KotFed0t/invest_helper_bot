package utils

import (
	"context"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v4"
)

func GetRequestIDFromCtx(ctx context.Context) string {
	rqID, ok := ctx.Value("rqID").(string)
	if !ok {
		return ""
	}
	return rqID
}

func CreateCtxWithRqID(c tele.Context) context.Context {
	rqId, ok := c.Get("rqID").(string)
	if !ok {
		return context.WithValue(context.Background(), "rqID", uuid.NewString())
	}
	return context.WithValue(context.Background(), "rqID", rqId)
}
