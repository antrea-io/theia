--Drop aggregated TAD columns
ALTER TABLE tadetector DROP COLUMN sourcePodNamespace String;
ALTER TABLE tadetector_local DROP COLUMN sourcePodNamespace String;
ALTER TABLE tadetector DROP COLUMN sourcePodLabels String;
ALTER TABLE tadetector_local DROP COLUMN sourcePodLabels String;
ALTER TABLE tadetector DROP COLUMN destinationPodNamespace String;
ALTER TABLE tadetector_local DROP COLUMN destinationPodNamespace String;
ALTER TABLE tadetector DROP COLUMN destinationPodLabels String;
ALTER TABLE tadetector_local DROP COLUMN destinationPodLabels String;
ALTER TABLE tadetector DROP COLUMN destinationServicePortName String;
ALTER TABLE tadetector_local DROP COLUMN destinationServicePortName String;
ALTER TABLE tadetector DROP COLUMN aggType String;
ALTER TABLE tadetector_local DROP COLUMN aggType String;
