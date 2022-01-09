package services

func HasIndicesWithNoReplica(service *OsClusterClient) (bool, error) {
	response, err := CatIndices(service)
	if err != nil {
		return false, err
	}
	for _, index := range response {
		if index.Rep == "" || index.Rep == "0" {
			return true, err
		}
	}
	return false, err
}
