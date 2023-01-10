create or replace function traffic_drop_%VERSION%(
    jobType STRING(20),
    detectionID STRING(40),
    endpoint STRING,
    direction STRING,
    date DATE,
    dropNumber NUMBER(20, 0)
)
returns table ( jobType STRING(20),
                detectionID STRING(40),
                timeCreated TIMESTAMP_NTZ,
                endpoint STRING,
	            direction STRING,
	            avgDrop NUMBER(25, 5),
	            stdevDrop NUMBER(25, 5),
                anomalyDropDate DATE,
	            anomalyDropNumber NUMBER(20, 0))
language python
runtime_version=3.8
packages = ('pandas')
imports=('@UDFS/traffic_drop_%VERSION%.zip')
handler='traffic_drop/traffic_drop_udf.TrafficDrop';
