--Alter table to drop new columns
ALTER TABLE flows
    DROP COLUMN l7ProtocolName,
    DROP COLUMN httpVals;
ALTER TABLE flows_local
    DROP COLUMN l7ProtocolName,
    DROP COLUMN httpVals;
