package utils

import (
	"context"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v4"
)

type rqIDKey struct{}

func GetRequestIDFromCtx(ctx context.Context) string {
	rqID, ok := ctx.Value(rqIDKey{}).(string)
	if !ok {
		return ""
	}
	return rqID
}

func CreateCtxWithRqID(c tele.Context) context.Context {
	rqId, ok := c.Get("rqID").(string)
	if !ok {
		return context.WithValue(context.Background(), rqIDKey{}, uuid.NewString())
	}
	return context.WithValue(context.Background(), rqIDKey{}, rqId)
}
