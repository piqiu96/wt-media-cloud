package model

// Omitted capabilities are the pre-Bilibili consumer, which can execute Baidu
// only. Never offer a new-platform task merely because its name is an API enum.
func SupportedPublicationPlatforms(in []string) ([]string, bool) {
	if in == nil {
		return []string{"baidu"}, true
	}
	if len(in) < 1 || len(in) > 2 {
		return nil, false
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p != "baidu" && p != "bilibili" || seen[p] {
			return nil, false
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, true
}
