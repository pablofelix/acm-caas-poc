package client

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	GVRClusterDeployment = schema.GroupVersionResource{
		Group: "hive.openshift.io", Version: "v1", Resource: "clusterdeployments",
	}
	GVRClusterImageSet = schema.GroupVersionResource{
		Group: "hive.openshift.io", Version: "v1", Resource: "clusterimagesets",
	}
	GVRManagedCluster = schema.GroupVersionResource{
		Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters",
	}
	GVRManagedClusterInfo = schema.GroupVersionResource{
		Group: "internal.open-cluster-management.io", Version: "v1beta1", Resource: "managedclusterinfos",
	}
	GVRManifestWork = schema.GroupVersionResource{
		Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks",
	}
	GVRPolicy = schema.GroupVersionResource{
		Group: "policy.open-cluster-management.io", Version: "v1", Resource: "policies",
	}
	GVRPlacementBinding = schema.GroupVersionResource{
		Group: "policy.open-cluster-management.io", Version: "v1", Resource: "placementbindings",
	}
	GVRPlacementRule = schema.GroupVersionResource{
		Group: "apps.open-cluster-management.io", Version: "v1", Resource: "placementrules",
	}
	GVRPlacement = schema.GroupVersionResource{
		Group: "cluster.open-cluster-management.io", Version: "v1beta1", Resource: "placements",
	}
	GVRKlusterletAddonConfig = schema.GroupVersionResource{
		Group: "agent.open-cluster-management.io", Version: "v1", Resource: "klusterletaddonconfigs",
	}
	GVRNamespace = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "namespaces",
	}
	GVRSecret = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "secrets",
	}
	GVRMultiClusterObservability = schema.GroupVersionResource{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Resource: "multiclusterobservabilities",
	}
	GVRDeployment = schema.GroupVersionResource{
		Group: "apps", Version: "v1", Resource: "deployments",
	}
	GVRService = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "services",
	}
	GVRPersistentVolumeClaim = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "persistentvolumeclaims",
	}
	GVRConfigurationPolicy = schema.GroupVersionResource{
		Group: "policy.open-cluster-management.io", Version: "v1", Resource: "configurationpolicies",
	}
	GVRManagedClusterSetBinding = schema.GroupVersionResource{
		Group: "cluster.open-cluster-management.io", Version: "v1beta2", Resource: "managedclustersetbindings",
	}
)
