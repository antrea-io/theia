package config

import "time"

const (
	FlowVisibilityNS        = "flow-visibility"
	K8sQuantitiesReg        = "^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	SparkImage              = "projects.registry.vmware.com/antrea/theia-policy-recommendation:latest"
	SparkImagePullPolicy    = "IfNotPresent"
	SparkAppFile            = "local:///opt/spark/work-dir/policy_recommendation_job.py"
	SparkServiceAccount     = "policy-recommendation-spark"
	SparkVersion            = "3.1.1"
	StatusCheckPollInterval = 5 * time.Second
	StatusCheckPollTimeout  = 60 * time.Minute
)
