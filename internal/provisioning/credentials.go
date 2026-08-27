package provisioning

type credentialRequest struct {
	SecretName string
	Namespace  string
	Policies   []iamPolicySpec
}

type iamPolicySpec struct {
	ServiceName  string
	ResourceType string
	Roles        []string
}

var ibmCloudCredentialRequests = []credentialRequest{
	{
		SecretName: "ibm-cloud-credentials",
		Namespace:  "openshift-cloud-controller-manager",
		Policies: []iamPolicySpec{
			{ResourceType: "resource-group", Roles: []string{"crn:v1:bluemix:public:iam::::role:Viewer"}},
			{ServiceName: "is", Roles: []string{
				"crn:v1:bluemix:public:iam::::role:Editor",
				"crn:v1:bluemix:public:iam::::role:Operator",
				"crn:v1:bluemix:public:iam::::role:Viewer",
			}},
		},
	},
	{
		SecretName: "ibmcloud-credentials",
		Namespace:  "openshift-machine-api",
		Policies: []iamPolicySpec{
			{ServiceName: "is", Roles: []string{
				"crn:v1:bluemix:public:iam::::role:Operator",
				"crn:v1:bluemix:public:iam::::role:Editor",
				"crn:v1:bluemix:public:iam::::role:Viewer",
			}},
			{ResourceType: "resource-group", Roles: []string{"crn:v1:bluemix:public:iam::::role:Viewer"}},
		},
	},
	{
		SecretName: "installer-cloud-credentials",
		Namespace:  "openshift-image-registry",
		Policies: []iamPolicySpec{
			{ServiceName: "cloud-object-storage", Roles: []string{
				"crn:v1:bluemix:public:iam::::role:Viewer",
				"crn:v1:bluemix:public:iam::::role:Operator",
				"crn:v1:bluemix:public:iam::::role:Editor",
				"crn:v1:bluemix:public:iam::::role:Administrator",
				"crn:v1:bluemix:public:iam::::serviceRole:Reader",
				"crn:v1:bluemix:public:iam::::serviceRole:Writer",
			}},
			{ResourceType: "resource-group", Roles: []string{"crn:v1:bluemix:public:iam::::role:Viewer"}},
		},
	},
	{
		SecretName: "cloud-credentials",
		Namespace:  "openshift-ingress-operator",
		Policies: []iamPolicySpec{
			{ServiceName: "internet-svcs", Roles: []string{
				"crn:v1:bluemix:public:iam::::serviceRole:Manager",
				"crn:v1:bluemix:public:iam::::serviceRole:Reader",
				"crn:v1:bluemix:public:iam::::serviceRole:Writer",
			}},
			{ServiceName: "dns-svcs", Roles: []string{
				"crn:v1:bluemix:public:iam::::serviceRole:Manager",
				"crn:v1:bluemix:public:iam::::serviceRole:Reader",
				"crn:v1:bluemix:public:iam::::serviceRole:Writer",
			}},
		},
	},
	{
		SecretName: "ibm-cloud-credentials",
		Namespace:  "openshift-cluster-csi-drivers",
		Policies: []iamPolicySpec{
			{ServiceName: "is", Roles: []string{
				"crn:v1:bluemix:public:iam::::role:Operator",
				"crn:v1:bluemix:public:iam::::role:Editor",
				"crn:v1:bluemix:public:iam::::role:Viewer",
			}},
			{ResourceType: "resource-group", Roles: []string{"crn:v1:bluemix:public:iam::::role:Viewer"}},
		},
	},
}
