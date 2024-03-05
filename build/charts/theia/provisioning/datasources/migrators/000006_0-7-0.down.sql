--Alter table to drop new columns
ALTER TABLE flows
    DROP COLUMN l7ProtocolName,
    DROP COLUMN httpVals;
ALTER TABLE flows_local
    DROP COLUMN l7ProtocolName,
    DROP COLUMN httpVals;
ALTER TABLE flows_policy_view
    DROP COLUMN l7ProtocolName String,
    DROP COLUMN httpVals String;
ALTER TABLE flows_policy_view_local
    DROP COLUMN l7ProtocolName String,
    DROP COLUMN httpVals String;
