--Create a table to store the Throughput Anomaly Detector results
CREATE TABLE IF NOT EXISTS tadetector_local (
    sourceIP String,
    sourceTransportPort UInt16,
    destinationIP String,
    destinationTransportPort UInt16,
    protocolIdentifier UInt16,
    flowStartSeconds DateTime,
    flowEndSeconds DateTime,
    throughputStandardDeviation Float64,
    algoType String,
    algoCalc Float64,
    throughput Float64,
    anomaly String,
    id String
) engine=ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
ORDER BY (flowStartSeconds);

CREATE TABLE IF NOT EXISTS tadetector AS tadetector_local
    engine=Distributed('{cluster}', default, tadetector_local, rand());
