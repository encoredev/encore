package cueutil

type EnvType string

const (
	EnvType_Production  EnvType = "production"
	EnvType_Development EnvType = "development"
	EnvType_Ephemeral   EnvType = "ephemeral"
	EnvType_Test        EnvType = "test"
)

type CloudType string

const (
	CloudType_AWS    CloudType = "aws"
	CloudType_Azure  CloudType = "azure"
	CloudType_GCP    CloudType = "gcp"
	CloudType_Encore CloudType = "encore"
	CloudType_Local  CloudType = "local"
)

type Meta struct {
	APIBaseURL string
	EnvName    string
	EnvType    EnvType
	CloudType  CloudType
}

func (m *Meta) ToTags() []string {
	if m == nil {
		return nil
	}

	return []string{
		tag("APIBaseURL", m.APIBaseURL),
		tag("EnvName", m.EnvName),
		tag("EnvType", m.EnvType),
		tag("CloudType", m.CloudType),
	}
}

func tag[T ~string](name string, value T) string {
	return name + "=" + string(value)
}
