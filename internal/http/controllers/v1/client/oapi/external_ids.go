package oapi

import "github.com/lunogram/platform/internal/store/subjects"

// ToParam converts an ExternalID to a subjects.ExternalIDParam.
func (id ExternalID) ToParam() subjects.ExternalIDParam {
	source := "default"
	if id.Source != nil && *id.Source != "" {
		source = *id.Source
	}
	p := subjects.ExternalIDParam{
		Source:     source,
		ExternalID: id.ExternalId,
	}
	if id.Metadata != nil {
		p.Metadata = *id.Metadata
	}
	return p
}

// ToParams converts a slice of ExternalID to a slice of subjects.ExternalIDParam.
func ToParams(ids []ExternalID) []subjects.ExternalIDParam {
	result := make([]subjects.ExternalIDParam, len(ids))
	for i, id := range ids {
		result[i] = id.ToParam()
	}
	return result
}
