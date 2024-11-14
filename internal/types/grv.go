package types

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	Annotations_DpuSf_Staus   = "k8s.v1.cni.cncf.io/network-status"
	Annotations_DpuSf_Network = "k8s.v1.cni.cncf.io/networks"

	DpuSfGv  = schema.GroupVersion{Group: "k8s.cni.cncf.io", Version: "v1"}
	DpuSfGvk = DpuSfGv.WithKind("NetworkAttachmentDefinition")
	DpuSfGvr = DpuSfGv.WithResource("network-attachment-definitions")
)
