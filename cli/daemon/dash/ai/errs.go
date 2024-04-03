package ai

type CodeType string

const (
	CodeTypeEndpoint CodeType = "endpoint"
	CodeTypeTypes    CodeType = "types"
)

type Pos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type ValidationError struct {
	Service  string   `json:"service"`
	Endpoint string   `json:"endpoint"`
	CodeType CodeType `json:"codeType"`
	Message  string   `json:"message"`
	Start    *Pos     `json:"start,omitempty"`
	End      *Pos     `json:"end,omitempty"`
}
