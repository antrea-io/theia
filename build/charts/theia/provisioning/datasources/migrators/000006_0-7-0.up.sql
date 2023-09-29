--Alter table to add new columns
ALTER TABLE flows
    ADD COLUMN appProtocolName String,
    ADD COLUMN httpVals String;
ALTER TABLE flows_local
    ADD COLUMN appProtocolName String,
    ADD COLUMN httpVals String;
