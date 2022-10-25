--Remove structured recommendations
--Add yamls column
ALTER TABLE recommendations ADD COLUMN yamls String;
ALTER TABLE recommendations_local ADD COLUMN yamls String;
--Copy old data and replace policy and kind by yamls
INSERT INTO recommendations_local (id, type, timeCreated, yamls)
SELECT id, type, timeCreated, arrayStringConcat(groupArray(policy), '\n---\n') AS yamls FROM recommendations_local GROUP BY id, type, timeCreated;
--Delete old data
ALTER TABLE recommendations_local DELETE WHERE yamls='';
--Drop yamls column
ALTER TABLE recommendations DROP COLUMN kind;
ALTER TABLE recommendations_local DROP COLUMN kind;
ALTER TABLE recommendations DROP COLUMN policy;
ALTER TABLE recommendations_local DROP COLUMN policy;
