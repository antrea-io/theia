--Add aggregated TAD columns
ALTER TABLE tadetector ADD COLUMN podNamespace String;
ALTER TABLE tadetector_local ADD COLUMN podNamespace String;
ALTER TABLE tadetector ADD COLUMN podLabels String;
ALTER TABLE tadetector_local ADD COLUMN podLabels String;
ALTER TABLE tadetector ADD COLUMN destinationServicePortName String;
ALTER TABLE tadetector_local ADD COLUMN destinationServicePortName String;
ALTER TABLE tadetector ADD COLUMN aggType String;
ALTER TABLE tadetector_local ADD COLUMN aggType String;
ALTER TABLE tadetector ADD COLUMN direction String;
ALTER TABLE tadetector_local ADD COLUMN direction String;
ALTER TABLE tadetector ADD COLUMN podName String;
ALTER TABLE tadetector_local ADD COLUMN podName String;
