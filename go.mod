module github.com/fusion-app/gateway

go 1.15

require (
	github.com/alecthomas/jsonschema v0.0.0-20210413112511-5c9c23bdc720
	github.com/eclipse/paho.mqtt.golang v1.3.3
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.3.0 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.18.0
	github.com/wI2L/jsondiff v0.1.0
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v12.0.0+incompatible
	kubevirt.io/client-go v0.33.0
	sigs.k8s.io/controller-runtime v0.7.0
)

replace k8s.io/client-go => k8s.io/client-go v0.19.2
