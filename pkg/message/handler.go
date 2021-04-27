package message

import (
	"encoding/json"
	ctrl "sigs.k8s.io/controller-runtime"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
)

var msgLogger = ctrl.Log.WithName("message")

type MsgHandler interface {
	Publish(*Message) error
}

var handlerCache = make(map[string]MsgHandler)

func NewMsgHandlerOrExist(spec monitorv1alpha1.MsgBackendSpec) MsgHandler {
	keyData, err := json.Marshal(spec)
	if err != nil {
		msgLogger.Error(err, "MsgBackendSpec serialize failed")
		return nil
	}
	key := string(keyData)

	if handler, exists := handlerCache[key]; exists {
		msgLogger.Info("Use exist MsgHandler", "handler", handler)
		return handler
	}
	if spec.MQTTBackend != nil {
		handler := NewMQTTMsgHandler(spec.MQTTBackend)
		handlerCache[key] = handler
		return handler
	}
	return nil
}
