package msg

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/jsonschema"
	"github.com/fusion-app/gateway/pkg/prom"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sync"

	monitorv1alpha1 "github.com/fusion-app/gateway/api/v1alpha1"
	"github.com/fusion-app/gateway/pkg/utils"
)

type MessageCache struct {
	Message *Message
	Metrics map[string]interface{}
}

type MessageStore struct {
	logger   logr.Logger
	handler  MsgHandler
	schemaID string
	// key: namespacedName
	cache map[string]*MessageCache
	mtx   sync.Mutex

	//stats
	pubCount uint64
}

func NewMsgStore(ref *monitorv1alpha1.ResourceMonitor) *MessageStore {
	return &MessageStore{
		logger:   ctrl.Log.WithName("store"),
		handler:  NewMsgHandlerOrExist(ref.Spec.MsgBackendSpec),
		schemaID: "",
		cache:    make(map[string]*MessageCache),
	}
}

func (s *MessageStore) OnResourceAdd(obj interface{}, u *unstructured.Unstructured) {
	if s.schemaID == "" {
		schemaID := utils.JSONSchemaID(u)
		schemaObj := jsonschema.Reflect(obj)
		schemaObj.Definitions["Extras"] = &jsonschema.Type{
			Type: "array",
			Extras: map[string]interface{}{
				"template": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"key": map[string]string{
							"type": "string",
						},
						"val": map[string]string{
							"type": "string",
						},
					},
				},
			},
		}
		schemaData, err := schemaObj.MarshalJSON()
		if err != nil {
			s.logger.Error(err, "Serialize JSON Schema failed")
			return
		}
		msg := &Message{
			Op:   RegisterSchema,
			Data: schemaData,
		}
		msgData, err := msg.MarshalJSON(nil)
		if err != nil {
			s.logger.Error(err, "Serialize Message failed")
		}
		if err = s.handler.Publish(msgData); err != nil {
			s.logger.Error(err, "Register JSON Schema failed")
			return
		}
		s.schemaID = schemaID
	}

	objRawData, err := json.Marshal(obj)
	if err != nil {
		s.logger.Error(err, "Build Message failed")
		return
	}
	msg := &Message{
		Op: NewResource,
		Meta: &ResourceMeta{
			SchemaID:  s.schemaID,
			Namespace: u.GetNamespace(),
			Name:      u.GetName(),
		},
		Data: objRawData,
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	key := cacheKey(s.schemaID, u.GetNamespace(), u.GetName())
	if oldCache, exists := s.cache[key]; exists {
		if msg.Equal(oldCache.Message) {
			return
		}
	}
	s.cache[key] = &MessageCache{
		Message: msg,
		Metrics: make(map[string]interface{}),
	}
	msgData, err := msg.MarshalJSON(nil)
	if err != nil {
		s.logger.Error(err, "Serialize Message failed")
	}
	_ = s.handler.Publish(msgData)
}

func (s *MessageStore) OnResourceUpdate(obj interface{}, u *unstructured.Unstructured) {
	objRawData, err := json.Marshal(obj)
	if err != nil {
		s.logger.Error(err, "Build Message failed")
		return
	}
	msg := &Message{
		Op: UpdateResource,
		Meta: &ResourceMeta{
			SchemaID:  s.schemaID,
			Namespace: u.GetNamespace(),
			Name:      u.GetName(),
		},
		Data: objRawData,
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	key := cacheKey(s.schemaID, u.GetNamespace(), u.GetName())
	oldCache, exists := s.cache[key]
	if exists {
		if msg.Equal(oldCache.Message) {
			return
		}
		msgData, err := msg.MarshalJSON(oldCache.Metrics)
		if err != nil {
			s.logger.Error(err, "Serialize Message failed")
		}
		_ = s.handler.Publish(msgData)
	} else {
		s.cache[key] = &MessageCache{
			Message: msg,
			Metrics: make(map[string]interface{}),
		}
		msgData, err := msg.MarshalJSON(nil)
		if err != nil {
			s.logger.Error(err, "Serialize Message failed")
		}
		_ = s.handler.Publish(msgData)
	}
}

func (s *MessageStore) OnMetricUpdate(r *prom.MetricResult) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	key := cacheKey(s.schemaID, r.ResNamespace, r.ResName)
	oldCache, exists := s.cache[key]
	if exists {
		if reflect.DeepEqual(oldCache.Metrics, r.Fields) {
			return
		}
		oldCache.Metrics = r.Fields
		msgData, err := oldCache.Message.MarshalJSON(r.Fields)
		if err != nil {
			s.logger.Error(err, "Serialize Message failed")
		}
		_ = s.handler.Publish(msgData)
	} else {
		msg := &Message{
			Op: UpdateResource,
			Meta: &ResourceMeta{
				SchemaID:  s.schemaID,
				Namespace: r.ResNamespace,
				Name:      r.ResName,
			},
		}
		s.cache[key] = &MessageCache{
			Message: msg,
			Metrics: r.Fields,
		}
		msgData, err := msg.MarshalJSON(r.Fields)
		if err != nil {
			s.logger.Error(err, "Serialize Message failed")
		}
		_ = s.handler.Publish(msgData)
	}
}

func (s *MessageStore) OnResourceDel(obj interface{}, u *unstructured.Unstructured) {
	objRawData, err := json.Marshal(obj.(ctrl.ObjectMeta))
	if err != nil {
		return
	}
	msg := &Message{
		Op: DelResource,
		Meta: &ResourceMeta{
			SchemaID:  s.schemaID,
			Namespace: u.GetNamespace(),
			Name:      u.GetName(),
		},
		Data: objRawData,
	}
	msgData, err := msg.MarshalJSON(nil)
	if err != nil {
		s.logger.Error(err, "Serialize Message failed")
	}
	_ = s.handler.Publish(msgData)
}

func cacheKey(schemaID, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", schemaID, namespace, name)
}
