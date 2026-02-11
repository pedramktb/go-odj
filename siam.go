package odj

import "encoding/json"

type SIAMGroupMembershipsDTO []string

// UnmarshalJSON reads an array of strings into a slice of strings
// but if the input is a single string
// it will be converted to a slice with one element.
// This fixes the inconsistency in the SIAM JWT format.
func (dto *SIAMGroupMembershipsDTO) UnmarshalJSON(data []byte) error {
	var singleString string
	if err := json.Unmarshal(data, &singleString); err == nil {
		*dto = SIAMGroupMembershipsDTO{singleString}
		return nil
	}

	var moreStrings []string
	if err := json.Unmarshal(data, &moreStrings); err != nil {
		return err
	}

	*dto = moreStrings
	return nil
}
