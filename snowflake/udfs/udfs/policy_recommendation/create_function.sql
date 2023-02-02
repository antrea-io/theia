create or replace function preprocessing_%VERSION%(
	job_type STRING(20),
	isolation_method NUMBER(1, 0),
	ns_allow_list STRING(10000),
	label_ignore_list STRING(10000),
	source_pod_namespace STRING(256),
	source_pod_labels STRING(10000),
	destination_ip STRING(50),
	destination_pod_namespace STRING(256),
	destination_pod_labels STRING(10000),
	destination_service_port_name STRING(256),
	destination_transport_port NUMBER(5, 0),
	protocol_identifier NUMBER(3, 0),
	flow_type NUMBER(3, 0)
)
returns table ( applied_to STRING,
                ingress STRING,
                egress STRING )
language python
runtime_version=3.8
imports=('@UDFS/policy_recommendation_%VERSION%.zip')
handler='policy_recommendation/preprocessing_udf.PreProcessing';

create or replace function policy_recommendation_%VERSION%(
       job_type STRING(20),
       recommendation_id STRING(40),
       isolation_method NUMBER(1, 0),
       ns_allow_list STRING(10000),
       applied_to STRING,
       ingress STRING,
       egress STRING
)
returns table ( job_type STRING(20),
                recommendation_id STRING(40),
                time_created TIMESTAMP_NTZ,
                yamls STRING )
language python
runtime_version=3.8
packages = ('six', 'python-dateutil', 'urllib3', 'requests', 'pyyaml')
imports=('@UDFS/policy_recommendation_%VERSION%.zip', '@UDFS/kubernetes.zip')
handler='policy_recommendation/policy_recommendation_udf.PolicyRecommendation';

create or replace function static_policy_recommendation_%VERSION%(
       job_type STRING(20),
       recommendation_id STRING(40),
       isolation_method NUMBER(1, 0),
       ns_allow_list STRING(10000)
)
returns table ( job_type STRING(20),
                recommendation_id STRING(40),
                time_created TIMESTAMP_NTZ,
                yamls STRING )
language python
runtime_version=3.8
packages = ('six', 'python-dateutil', 'urllib3', 'requests', 'pyyaml')
imports=('@UDFS/policy_recommendation_%VERSION%.zip', '@UDFS/kubernetes.zip')
handler='policy_recommendation/static_policy_recommendation_udf.StaticPolicyRecommendation';
