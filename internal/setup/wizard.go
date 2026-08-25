package setup

import "sort"

type StepStatus string

const (
	Pending StepStatus = "PENDING"
	Ready   StepStatus = "READY"
	Skipped StepStatus = "SKIPPED"
	Failed  StepStatus = "FAILED"
)

type WizardSnapshot struct {
	Admin, Storage, Network          bool
	MQTTEnabled, MQTTHealthy         bool
	HAEnabled, HAHealthy             bool
	FrigateEnabled, FrigateHealthy   bool
	CameraCount                      int
	IntercomCount                    int
	AIEnabled, AIHealthy             bool
	TelegramEnabled, TelegramHealthy bool
	BackupEnabled, BackupHealthy     bool
}
type WizardStep struct {
	ID, Title           string
	Status              StepStatus
	Required            bool
	Reason, ActionRoute string
}
type WizardState struct {
	Steps         []WizardStep `json:"steps"`
	Complete      bool         `json:"complete"`
	ReadyCount    int          `json:"ready_count"`
	RequiredCount int          `json:"required_count"`
}

func EvaluateWizard(s WizardSnapshot) WizardState {
	steps := []WizardStep{
		{ID: "server", Title: "Сервер и администратор", Required: true, ActionRoute: "#system"}, {ID: "storage", Title: "Хранилище", Required: true, ActionRoute: "#system"}, {ID: "network", Title: "Сеть и camera CIDR", Required: true, ActionRoute: "#system"}, {ID: "mqtt", Title: "MQTT", Required: s.MQTTEnabled, ActionRoute: "#settings"}, {ID: "home_assistant", Title: "Home Assistant", Required: s.HAEnabled, ActionRoute: "#settings"}, {ID: "frigate", Title: "Frigate / go2rtc", Required: s.FrigateEnabled, ActionRoute: "#settings"}, {ID: "cameras", Title: "Камеры", Required: true, ActionRoute: "#cameras"}, {ID: "intercom", Title: "Домофон", Required: false, ActionRoute: "#entrance"}, {ID: "ai", Title: "Local AI", Required: false, ActionRoute: "#settings"}, {ID: "telegram", Title: "Telegram", Required: false, ActionRoute: "#settings"}, {ID: "backup", Title: "Backup / restore", Required: s.BackupEnabled, ActionRoute: "#settings"}}
	good := map[string]bool{"server": s.Admin, "storage": s.Storage, "network": s.Network, "mqtt": s.MQTTHealthy, "home_assistant": s.HAHealthy, "frigate": s.FrigateHealthy, "cameras": s.CameraCount > 0, "intercom": s.IntercomCount > 0, "ai": s.AIHealthy, "telegram": s.TelegramHealthy, "backup": s.BackupHealthy}
	enabled := map[string]bool{"mqtt": s.MQTTEnabled, "home_assistant": s.HAEnabled, "frigate": s.FrigateEnabled, "ai": s.AIEnabled, "telegram": s.TelegramEnabled, "backup": s.BackupEnabled, "intercom": s.IntercomCount > 0}
	out := WizardState{Steps: steps, Complete: true}
	for i := range out.Steps {
		st := &out.Steps[i]
		if st.Required {
			out.RequiredCount++
		}
		if optionalKnown(st.ID) && !enabled[st.ID] {
			st.Status = Skipped
			st.Reason = "не включено в текущей конфигурации"
			continue
		}
		if good[st.ID] {
			st.Status = Ready
			out.ReadyCount++
			continue
		}
		st.Status = Pending
		st.Reason = "требуется настройка или успешная проверка"
		if st.Required {
			out.Complete = false
		}
	}
	sort.SliceStable(out.Steps, func(i, j int) bool { return i < j })
	return out
}
func optionalKnown(id string) bool {
	switch id {
	case "mqtt", "home_assistant", "frigate", "intercom", "ai", "telegram", "backup":
		return true
	}
	return false
}
