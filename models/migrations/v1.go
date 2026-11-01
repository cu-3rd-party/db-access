package migrations

import "xorm.io/xorm"

func v1CreateAccessTables(x *xorm.Engine) error {
	type Account struct {
		ID          int64  `xorm:"pk autoincr"`
		Login       string `xorm:"VARCHAR(8) NOT NULL UNIQUE"`
		CreatedUnix int64  `xorm:"created"`
		RotatedUnix int64
	}
	type WriteGrant struct {
		ID          int64  `xorm:"pk autoincr"`
		AccountID   int64  `xorm:"NOT NULL UNIQUE(account_db)"`
		DBName      string `xorm:"VARCHAR(63) NOT NULL UNIQUE(account_db)"`
		ExpiresUnix int64  `xorm:"NOT NULL INDEX"`
	}
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
	return x.Sync(new(Account), new(WriteGrant), new(AccessLog))
}
