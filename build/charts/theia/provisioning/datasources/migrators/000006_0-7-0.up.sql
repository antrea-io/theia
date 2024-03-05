--Alter table to add new columns
ALTER TABLE flows
    ADD COLUMN l7ProtocolName String,
    ADD COLUMN httpVals String;
ALTER TABLE flows_local
    ADD COLUMN l7ProtocolName String,
    ADD COLUMN httpVals String;
