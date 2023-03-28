--Drop aggregated TAD columns
ALTER TABLE tadetector DROP COLUMN podNamespace String;
ALTER TABLE tadetector_local DROP COLUMN podNamespace String;
ALTER TABLE tadetector DROP COLUMN podLabels String;
ALTER TABLE tadetector_local DROP COLUMN podLabels String;
ALTER TABLE tadetector DROP COLUMN destinationServicePortName String;
ALTER TABLE tadetector_local DROP COLUMN destinationServicePortName String;
ALTER TABLE tadetector DROP COLUMN aggType String;
ALTER TABLE tadetector_local DROP COLUMN aggType String;
ALTER TABLE tadetector DROP COLUMN direction String;
ALTER TABLE tadetector_local DROP COLUMN direction String;
ALTER TABLE tadetector DROP COLUMN podName String;
ALTER TABLE tadetector_local DROP COLUMN podName String;
