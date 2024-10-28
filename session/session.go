package session

import (
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/zekihan/sdash/config"
	"time"
)

type Cookie struct {
	ID            string `json:"id"`
	Created       time.Time
	ExpireTime    time.Time
	Authenticated bool
}

func (s *Cookie) IsExpired() bool {
	return s.ExpireTime.Before(time.Now())
}

func NewCookie() *Cookie {
	id, err := uuid.NewV7()
	if err != nil {
		log.Fatalf("Failed to generate session ID")
	}
	now := time.Now()
	return &Cookie{
		ID:            id.String(),
		Created:       now,
		ExpireTime:    now.Add(config.GetConfig().WebConfig.SessionTimeout),
		Authenticated: false,
	}
}
