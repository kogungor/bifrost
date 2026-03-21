package adapters

// All returns all registered adapters.
func All() []Adapter {
	return []Adapter{
		newClaudeCode(),
		newOpenCode(),
	}
}

// Get returns an adapter by name, or nil if not found.
func Get(name string) Adapter {
	for _, a := range All() {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// Installed returns only adapters whose tools are detected on the system.
func Installed() []Adapter {
	var result []Adapter
	for _, a := range All() {
		if a.IsInstalled() {
			result = append(result, a)
		}
	}
	return result
}
