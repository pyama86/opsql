package definition

type Definition struct {
	Version    int               `yaml:"version"`
	Params     map[string]string `yaml:"params"`
	Operations []Operation       `yaml:"operations"`
}

type Operation struct {
	ID              string                   `yaml:"id"`
	Description     string                   `yaml:"description"`
	Type            string                   `yaml:"type"`
	SQL             string                   `yaml:"sql"`
	Expected        []map[string]interface{} `yaml:"expected,omitempty"`
	ExpectedChanges map[string]int           `yaml:"expected_changes,omitempty"`
}

type Report struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Result      interface{} `json:"result"`
	Pass        bool        `json:"pass"`
	Message     string      `json:"message"`
}

const (
	TypeSelect = "select"
	TypeInsert = "insert"
	TypeUpdate = "update"
	TypeDelete = "delete"
)

var AllowedTypes = []string{TypeSelect, TypeInsert, TypeUpdate, TypeDelete}
