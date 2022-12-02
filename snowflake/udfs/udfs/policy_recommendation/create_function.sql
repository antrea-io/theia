create or replace function preprocessing_%VERSION%(
	jobType STRING(20),
	isolationMethod NUMBER(1, 0),
	nsAllowList STRING(10000),
	labelIgnoreList STRING(10000),
	sourcePodNamespace STRING(256),
	sourcePodLabels STRING(10000),
	destinationIP STRING(50),
	destinationPodNamespace STRING(256),
	destinationPodLabels STRING(10000),
	destinationServicePortName STRING(256),
	destinationTransportPort NUMBER(5, 0),
	protocolIdentifier NUMBER(3, 0),
	flowType NUMBER(3, 0)
)
returns table ( appliedTo STRING,
                ingress STRING,
                egress STRING )
language python
runtime_version=3.8
imports=('@UDFS/policy_recommendation_%VERSION%.zip')
handler='policy_recommendation/preprocessing_udf.PreProcessing';

create or replace function policy_recommendation_%VERSION%(
       jobType STRING(20),
       recommendationId STRING(40),
       isolationMethod NUMBER(1, 0),
       nsAllowList STRING(10000),
       appliedTo STRING,
       ingress STRING,
       egress STRING
)
returns table ( jobType STRING(20),
                recommendationId STRING(40),
                timeCreated TIMESTAMP_NTZ,
                yamls STRING )
language python
runtime_version=3.8
packages = ('six', 'python-dateutil', 'urllib3', 'requests', 'pyyaml')
imports=('@UDFS/policy_recommendation_%VERSION%.zip', '@UDFS/kubernetes.zip')
handler='policy_recommendation/policy_recommendation_udf.PolicyRecommendation';

create or replace function static_policy_recommendation_%VERSION%(
       jobType STRING(20),
       recommendationId STRING(40),
       isolationMethod NUMBER(1, 0),
       nsAllowList STRING(10000)
)
returns table ( jobType STRING(20),
                recommendationId STRING(40),
                timeCreated TIMESTAMP_NTZ,
                yamls STRING )
language python
runtime_version=3.8
packages = ('six', 'python-dateutil', 'urllib3', 'requests', 'pyyaml')
imports=('@UDFS/policy_recommendation_%VERSION%.zip', '@UDFS/kubernetes.zip')
handler='policy_recommendation/static_policy_recommendation_udf.StaticPolicyRecommendation';
