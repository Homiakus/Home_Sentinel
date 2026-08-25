package capability

func NewTriggerDescriptor(id, version, providerID, integrationID string, category Category, title string, permission Permission) (Descriptor, error) {
	return NewDescriptor(KindTrigger, id, version, providerID, integrationID, category, title, permission)
}
