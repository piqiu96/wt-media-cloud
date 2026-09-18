package repository

func validMode(mode string) bool {
	return mode == "local" || mode == "cloud"
}

func validStatus(status string) bool {
	return status == "online" || status == "draining"
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
