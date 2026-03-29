---
description: Remove Antwort (Background)
---

# Uninstall Antwort (Background)

## Prerequisites Check

Verify AWS and ROSA authentication.

## Confirmation

Ask the user to confirm. Warn that PostgreSQL data (queued jobs, responses) will be lost if co-located database is removed.

## Removal

```bash
oc delete deployment antwort-gateway -n ${NAMESPACE} --ignore-not-found
oc delete deployment antwort-worker -n ${NAMESPACE} --ignore-not-found
oc delete service antwort -n ${NAMESPACE} --ignore-not-found
oc delete route antwort -n ${NAMESPACE} --ignore-not-found
oc delete configmap antwort-config -n ${NAMESPACE} --ignore-not-found
oc delete statefulset postgres -n ${NAMESPACE} --ignore-not-found
oc delete pvc -l app=postgres -n ${NAMESPACE} --ignore-not-found
```

## Post-Uninstall

Confirm removal completed.
