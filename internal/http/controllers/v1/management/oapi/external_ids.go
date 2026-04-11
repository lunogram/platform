package oapi

// ExternalIDParam represents an external identifier parameter for store operations.
type ExternalIDParam struct {
	Source     string         `json:"source"`
	ExternalID string         `json:"external_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ToParam converts an ExternalID to an ExternalIDParam.
func (id ExternalID) ToParam() ExternalIDParam {
	source := "default"
	if id.Source != nil && *id.Source != "" {
		source = *id.Source
	}
	p := ExternalIDParam{
		Source:     source,
		ExternalID: id.ExternalId,
	}
	if id.Metadata != nil {
		p.Metadata = *id.Metadata
	}
	return p
}

// ToParams converts a slice of ExternalID to a slice of ExternalIDParam.
func ToParams(ids []ExternalID) []ExternalIDParam {
	result := make([]ExternalIDParam, len(ids))
	for i, id := range ids {
		result[i] = id.ToParam()
	}
	return result
}
