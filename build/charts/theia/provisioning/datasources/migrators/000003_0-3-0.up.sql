--Support structured recommendations
--Add policy and kind column
ALTER TABLE recommendations ADD COLUMN kind String;
ALTER TABLE recommendations_local ADD COLUMN kind String;
ALTER TABLE recommendations ADD COLUMN policy String;
ALTER TABLE recommendations_local ADD COLUMN policy String;
--Copy old data and replace yamls column by policy and kind
INSERT INTO recommendations_local (id, type, timeCreated, policy)
SELECT id, type, timeCreated, arrayJoin(splitByString('---\n', yamls)) AS policy FROM recommendations_local WHERE kind='';
ALTER TABLE recommendations_local UPDATE kind='knp' WHERE policy LIKE '%networking.k8s.io/v1%NetworkPolicy%';
ALTER TABLE recommendations_local UPDATE kind='anp' WHERE policy LIKE '%crd.antrea.io/v1alpha1%NetworkPolicy%';
ALTER TABLE recommendations_local UPDATE kind='acnp' WHERE policy LIKE '%ClusterNetworkPolicy%';
ALTER TABLE recommendations_local UPDATE kind='acg' WHERE policy LIKE '%ClusterGroup%';
--Delete old data
ALTER TABLE recommendations_local DELETE WHERE policy='';
--Drop yamls column
ALTER TABLE recommendations DROP COLUMN yamls;
ALTER TABLE recommendations_local DROP COLUMN yamls;
