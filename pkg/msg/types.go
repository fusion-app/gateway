package msg

import (
	"encoding/json"
	"reflect"

	"github.com/wI2L/jsondiff"
)

type Message struct {
	Op   ResourceOp    `json:"op"`
	Meta *ResourceMeta `json:"meta,omitempty"`
	Data []byte        `json:"data,omitempty"`
}

type ResourceMeta struct {
	SchemaID  string `json:"schema_id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type ResourceOp string

const (
	RegisterSchema ResourceOp = "RegisterSchema"
	NewResource    ResourceOp = "New"
	DelResource    ResourceOp = "Delete"
	UpdateResource ResourceOp = "Update"
)

func (m *Message) MarshalJSON(extras map[string]interface{}) ([]byte, error) {
	newData := make([]byte, len(m.Data))
	copy(newData, m.Data)
	if extras != nil && len(extras) > 0 {
		extraBytes, err := json.Marshal(struct {
			Extras map[string]interface{} `json:"extras,omitempty"`
		}{extras})
		if err != nil {
			return nil, err
		}
		if len(newData) == 2 {
			newData = extraBytes
		} else {
			newData[len(newData)-1] = ','
			newData = append(newData, extraBytes[1:]...)
		}
	}

	return json.Marshal(&Message{
		Op:   m.Op,
		Meta: m.Meta,
		Data: newData,
	})
}

func (m *Message) Equal(other *Message) bool {
	if m.Op != other.Op || !reflect.DeepEqual(m.Meta, other.Meta) || (len(m.Data) == 0 || len(other.Data) == 0) {
		return false
	}

	diff, err := jsondiff.CompareJSON(m.Data, other.Data)
	return err == nil && len(diff) == 0
}
