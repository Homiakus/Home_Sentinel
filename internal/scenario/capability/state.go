package capability

func NewStateDescriptor(id, version, providerID, integrationID string, category Category, title string, permission Permission) (Descriptor, error) {
	return NewDescriptor(KindState, id, version, providerID, integrationID, category, title, permission)
}
