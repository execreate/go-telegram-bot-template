package tables

type ConfigKey string

const (
	MyChannelID ConfigKey = "my_channel_id"
	MyGroupID   ConfigKey = "my_group_id"
)

// Config keeps config values
type Config struct {
	SoftDeleteModel

	Key   ConfigKey `db:"key"`
	Value string    `db:"value"`
}
