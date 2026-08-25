package capability

import "testing"

func TestConstructorsReturnValidDescriptors(t *testing.T) {
	constructors := []func() (Descriptor, error){
		func() (Descriptor, error) {
			return NewTriggerDescriptor("camera.person.detected", "1.0.0", "rtsp", "rtsp-main", "security", "Person detected", "camera.read")
		},
		func() (Descriptor, error) {
			return NewActionDescriptor("camera.snapshot", "1.0.0", "rtsp", "rtsp-main", "evidence", "Take snapshot", "camera.snapshot")
		},
		func() (Descriptor, error) {
			return NewStateDescriptor("camera.online", "1.0.0", "rtsp", "rtsp-main", "health", "Camera online", "camera.read")
		},
	}
	for i, constructor := range constructors {
		descriptor, err := constructor()
		if err != nil {
			t.Fatalf("constructor %d: %v", i, err)
		}
		if err := descriptor.Validate(); err != nil {
			t.Fatalf("constructor %d returned invalid descriptor: %v", i, err)
		}
		if !descriptor.Available || descriptor.Health != HealthHealthy {
			t.Fatalf("constructor %d returned unusable defaults", i)
		}
	}
}

func TestConstructorFailsFastOnIncompleteDescriptor(t *testing.T) {
	if _, err := NewActionDescriptor("camera.snapshot", "1.0.0", "", "rtsp-main", "evidence", "Take snapshot", "camera.snapshot"); err == nil {
		t.Fatal("constructor accepted missing provider id")
	}
}
