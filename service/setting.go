package service

import (
	"context"
	"time"
)

type Settings struct {
	Title    string
	Tagline  string
	TimeZone *time.Location
}

type SettingRepository interface {
	Load(ctx context.Context) (settings *Settings, err error)
	Save(ctx context.Context, settings *Settings) (err error)
}
