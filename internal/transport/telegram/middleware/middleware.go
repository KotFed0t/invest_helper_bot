package middleware

import (
	"fmt"
	"log/slog"
	"time"

	tele "gopkg.in/telebot.v4"
	"github.com/google/uuid"
)

func Logger() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			now := time.Now()
			
			rqID := uuid.NewString()
			c.Set("rqID", rqID)

			slog.Info(
				"start request",
				slog.String("rqID", rqID),
				// slog.Any("update", c.Update()),
			)

			defer func() {
				slog.Info(
					"request finished",
					slog.String("rqID", rqID),
					slog.String("request duration", fmt.Sprintf("%.2fs", time.Since(now).Seconds())),
				)
			}()
			
			return next(c)
		}
	}
}