package vusmartmaps

import (
	"fmt"

	"go.elastic.co/apm/model"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// EncodeResourceMetadata encodes a metadata line from resource, writing to w.
func EncodeResourceMetadata(resource pcommon.Resource, w *fastjson.Writer) (err error) {
	var agent model.Agent
	var service model.Service
	var serviceNode model.ServiceNode
	var serviceLanguage model.Language
	var system model.System
	var k8s model.Kubernetes
	var k8sPod model.KubernetesPod
	var labels model.IfaceMap

	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case conventions.AttributeServiceName:
			service.Name = cleanServiceName(v.StringVal())
		case conventions.AttributeServiceVersion:
			service.Version = truncate(v.StringVal())
		case conventions.AttributeServiceInstanceID:
			serviceNode.ConfiguredName = truncate(v.StringVal())
			service.Node = &serviceNode
		case conventions.AttributeDeploymentEnvironment:
			service.Environment = truncate(v.StringVal())

		case conventions.AttributeTelemetrySDKName:
			agent.Name = truncate(v.StringVal())
		case conventions.AttributeTelemetrySDKLanguage:
			serviceLanguage.Name = truncate(v.StringVal())
			service.Language = &serviceLanguage
		case conventions.AttributeTelemetrySDKVersion:
			agent.Version = truncate(v.StringVal())

		case conventions.AttributeK8SNamespaceName:
			k8s.Namespace = truncate(v.StringVal())
			system.Kubernetes = &k8s
		case conventions.AttributeK8SPodName:
			k8sPod.Name = truncate(v.StringVal())
			k8s.Pod = &k8sPod
			system.Kubernetes = &k8s
		case conventions.AttributeK8SPodUID:
			k8sPod.UID = truncate(v.StringVal())
			k8s.Pod = &k8sPod
			system.Kubernetes = &k8s

		case conventions.AttributeHostName:
			system.Hostname = truncate(v.StringVal())

		default:
			labels = append(labels, model.IfaceMapItem{
				Key:   cleanLabelKey(k),
				Value: ifaceAttributeValue(v),
			})
		}
		return true
	})

	if service.Name == "" {
		// service.name is a required field.
		service.Name = "unknown"
	}
	if agent.Name == "" {
		// service.agent.name is a required field.
		agent.Name = "otlp"
	}
	if agent.Version == "" {
		// service.agent.version is a required field.
		agent.Version = "unknown"
	}
	if serviceLanguage.Name != "" {
		agent.Name = fmt.Sprintf("%s/%s", agent.Name, serviceLanguage.Name)
	}
	service.Agent = &agent

	w.RawString(`{"metadata":{`)
	w.RawString(`"service":`)
	if err := service.MarshalFastJSON(w); err != nil {
		return err
	}
	if system != (model.System{}) {
		w.RawString(`,"system":`)
		if err := system.MarshalFastJSON(w); err != nil {
			return err
		}
	}
	if len(labels) > 0 {
		w.RawString(`,"labels":`)
		if err := labels.MarshalFastJSON(w); err != nil {
			return err
		}
	}
	w.RawString("}}\n")
	return nil
}
