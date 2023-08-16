package tables

import (
	"database/sql/driver"
	"gorm.io/gorm"
)

type ConfigKey string

const (
	MyChannelID ConfigKey = "my_channel_id"
	MyGroupID   ConfigKey = "my_group_id"
)

func (ck *ConfigKey) Scan(value interface{}) error {
	*ck = ConfigKey(value.(string))
	return nil
}

func (ck ConfigKey) Value() (driver.Value, error) {
	return string(ck), nil
}

// Config keeps config values
type Config struct {
	gorm.Model

	Key   ConfigKey `gorm:"size:250;uniqueIndex;not null"`
	Value string    `gorm:"size:250;not null"`
}
