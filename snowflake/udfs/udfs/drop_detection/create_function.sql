create or replace function drop_detection_%VERSION%(
    job_type STRING(20),
    detection_id STRING(40),
    endpoint STRING,
    direction STRING,
    date DATE,
    drop_number NUMBER(20, 0)
)
returns table (
    job_type STRING(20),
    detection_id STRING(40),
    time_created TIMESTAMP_NTZ,
    endpoint STRING,
    direction STRING,
    avg_drop NUMBER(25, 5),
    stdev_drop NUMBER(25, 5),
    anomaly_drop_date DATE,
    anomaly_drop_number NUMBER(20, 0)
)
language python
runtime_version=3.8
packages = ('pandas')
imports=('@UDFS/drop_detection_%VERSION%.zip')
handler='drop_detection/drop_detection_udf.DropDetection';
