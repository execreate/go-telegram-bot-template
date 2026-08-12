package tables

// Config keeps config values
type Config struct {
	SoftDeleteModel

	Key   string `db:"key"`
	Value string `db:"value"`
}
