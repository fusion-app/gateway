package message

type Message struct {
	Op   ResourceOp    `json:"op"`
	Meta *ResourceMeta `json:"meta"`
	Data []byte        `json:"data"`
}

type ResourceMeta struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type ResourceOp string

const (
	NewResource    ResourceOp = "New"
	DelResource    ResourceOp = "Delete"
	UpdateResource ResourceOp = "Update"
)
