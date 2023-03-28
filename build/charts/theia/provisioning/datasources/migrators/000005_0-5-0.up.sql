--Add aggregated TAD columns
ALTER TABLE tadetector ADD COLUMN sourcePodNamespace String;
ALTER TABLE tadetector_local ADD COLUMN sourcePodNamespace String;
ALTER TABLE tadetector ADD COLUMN sourcePodLabels String;
ALTER TABLE tadetector_local ADD COLUMN sourcePodLabels String;
ALTER TABLE tadetector ADD COLUMN destinationPodNamespace String;
ALTER TABLE tadetector_local ADD COLUMN destinationPodNamespace String;
ALTER TABLE tadetector ADD COLUMN destinationPodLabels String;
ALTER TABLE tadetector_local ADD COLUMN destinationPodLabels String;
ALTER TABLE tadetector ADD COLUMN destinationServicePortName String;
ALTER TABLE tadetector_local ADD COLUMN destinationServicePortName String;
ALTER TABLE tadetector ADD COLUMN aggType String;
ALTER TABLE tadetector_local ADD COLUMN aggType String;
