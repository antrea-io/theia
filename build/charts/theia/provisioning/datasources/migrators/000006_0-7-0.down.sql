--Alter table to drop new columns
ALTER TABLE flows
    DROP COLUMN appProtocolName,
    DROP COLUMN httpVals;
ALTER TABLE flows_local
    DROP COLUMN appProtocolName,
    DROP COLUMN httpVals;
