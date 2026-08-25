package capability

func NewActionDescriptor(id, version, providerID, integrationID string, category Category, title string, permission Permission) (Descriptor, error) {
	return NewDescriptor(KindAction, id, version, providerID, integrationID, category, title, permission)
}
