package message

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	ctrl "sigs.k8s.io/controller-runtime"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
)

var mqttLogger = ctrl.Log.WithName("mqtt")

type MQTTMsgHandler struct {
	Client     mqtt.Client
	topic      string
	pubTimeout time.Duration
}

func NewMQTTMsgHandler(spec *monitorv1alpha1.MQTTBackendSpec) *MQTTMsgHandler {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", spec.Host, spec.Port))
	opts.SetClientID("k8s-gateway")
	if spec.Username != "" {
		opts.SetUsername(spec.Username)
	}
	if spec.Password != "" {
		opts.SetPassword(spec.Password)
	}
	opts.OnConnect = func(client mqtt.Client) {
		mqttLogger.Info("Connected")
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		mqttLogger.Error(err, "Connection lost")
	}
	//opts.DefaultPublishHandler = func(Client mqtt.Client, msg mqtt.Message) {
	//	mqttLogger.WithValues("topic", msg.Topic(), "payload", msg.Payload()).Info("Received message")
	//}
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	//go func() {
	//	if token := client.Subscribe(spec.Topic, 0, func(client mqtt.Client, msg mqtt.Message) {
	//		mqttLogger.Info("Receive message", "msg", msg.Payload())
	//	}); token.Wait() && token.Error() != nil {
	//		mqttLogger.Error(token.Error(), "Subscribe failed")
	//	}
	//}()
	return &MQTTMsgHandler{
		Client:     client,
		topic:      spec.Topic,
		pubTimeout: time.Second * 3,
	}
}

func (h *MQTTMsgHandler) Publish(msg *Message) error {
	msgData, err := json.Marshal(msg)
	if err != nil {
		mqttLogger.Error(err, "Serialize message failed")
		return err
	}
	token := h.Client.Publish(h.topic, 0, false, msgData)
	if !token.WaitTimeout(h.pubTimeout) {
		mqttLogger.Error(token.Error(), "Publish failed")
		return token.Error()
	}
	mqttLogger.Info("Publish success", "op", msg.Op, "meta", msg.Meta, "data", string(msg.Data))
	return nil
}
