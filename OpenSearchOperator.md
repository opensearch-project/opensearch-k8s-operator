# OpenSearch Operator

## Nötige Änderungen
- Custom Docker Image für Repository Azure Plugin
- opensearch-lifecycle-management um Snapshot Policies erweitern
- Recoverability mit PVCs sicherstellen (ggf. implementieren)

## Data Import

- Snapshots von OpenSearch 1 aus Azure Blob Storage in 2. Index (rename) wiederherstellen
- Merge zwischen altem und neuen Index durch [Reindexing](https://opensearch.org/docs/1.3/opensearch/snapshot-restore/#conflicts-and-compatibility)

## TODOs für den Operator

### Bugs
- Keystore befüllen ([GitHub Issue](https://github.com/Opster/opensearch-k8s-operator/issues/300))
- Index Policies (vgl. security-config)
- Snapshot Policies (vgl. security-config)
- OpensearchRole Cluster UID fetch ([Merged but not released](https://github.com/Opster/opensearch-k8s-operator/pull/277))
- Neustart bei Änderung von Opensearch Dashboards config
- Backend Roles
- secrets bei additional config
- Opensearch apply während startup
- Probes für kube rbac proxy
- pluginsList startet pods nicht neu

### Verbesserung
- Inkonsistenz zwischen README und /samples Beispiel
- security - auth
- tls enable
- Enabling execution of install_demo_configuration.sh for OpenSearch Security Plugin
- Resource Limits für Operator
- Doku vm_map_count
- Existierende PVCs blockieren Startup
- Opensearch Config auf map[string]interface{}
- Missing secrets DARF NICHT starten 
- OSD config als secret
- Wenn das Admin Secret beim Startup fehlt, keine erfolgreiche Recovery

### Doku erweitern
- Anzahl Replicas dokumentieren
- cluster_manager role (Noch nicht im Release)
- PVCs löschen (Issue)


# TODOS für mich
- Doku erweitern
- PVCs anschauen