// Package accesslog stores the service access log. It never holds request bodies,
// cookies or response bodies, so passwords cannot end up in it.
package accesslog

import (
	"context"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/models/db"
)

func init() { db.RegisterModel(new(AccessLog)) }

// AccessLog is one served HTTP request.
type AccessLog struct {
	ID             int64  `xorm:"pk autoincr"`
	TimeUnixMilli  int64  `xorm:"NOT NULL INDEX"`
	IP             string `xorm:"VARCHAR(64) INDEX"`
	RemoteAddr     string `xorm:"VARCHAR(64)"`
	ForwardedFor   string `xorm:"VARCHAR(256)"`
	Login          string `xorm:"VARCHAR(8) INDEX"`
	Method         string `xorm:"VARCHAR(16)"`
	Host           string `xorm:"VARCHAR(256)"`
	Path           string `xorm:"VARCHAR(1024)"`
	Query          string `xorm:"VARCHAR(1024)"`
	Proto          string `xorm:"VARCHAR(16)"`
	Status         int
	Bytes          int64
	DurationMillis int64
	UserAgent      string `xorm:"VARCHAR(512)"`
	Referer        string `xorm:"VARCHAR(512)"`
	AcceptLanguage string `xorm:"VARCHAR(128)"`
}

// Insert stores an entry.
func Insert(ctx context.Context, e *AccessLog) error {
	_, err := db.GetEngine(ctx).Insert(e)
	return err
}

// DeleteOlderThan removes entries older than t and returns how many.
func DeleteOlderThan(ctx context.Context, t time.Time) (int64, error) {
	return db.GetEngine(ctx).Where("time_unix_milli < ?", t.UnixMilli()).Delete(new(AccessLog))
}
