package message

import (
	"testing"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
)

func TestMQTTMsgHandler_Publish(t *testing.T) {
	handler := NewMQTTMsgHandler(&monitorv1alpha1.MQTTBackendSpec{
		Host: "test.mosquitto.org",
		Port: 1883,
		Topic: "udogateway",
	})
	err := handler.Publish(&Message{
		Op: NewResource,
		Meta: &ResourceMeta{
			Name: "test",
			Namespace: "default",
		},
	})
	if err != nil {
		t.Errorf("Pub failed")
	}
}
