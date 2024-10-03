package parser

type Raw struct{}

func (ra Raw) Apply(data [][]byte) (ParsedData, error) {
	parsedData := ParsedData{
		Data: map[string]interface{}{
			"rawData": string(data[0]),
		},
	}
	if len(data) > 1 {
		decodedMetadata, err := DecodeMetadata(data[1])
		if err != nil {
			return parsedData, err
		}

		for key, value := range decodedMetadata {
			if _, exists := parsedData.Data[key]; !exists {
				parsedData.Data[key] = value
			}
		}
	}
	return parsedData, nil
}
