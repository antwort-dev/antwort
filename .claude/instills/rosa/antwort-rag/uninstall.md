---
description: Remove Antwort (RAG)
---

# Uninstall Antwort (RAG)

## Prerequisites Check

Verify AWS and ROSA authentication.

## Confirmation

Ask the user to confirm. Warn that uploaded files and vector store data will be lost.

## Removal

```bash
oc delete deployment antwort -n ${NAMESPACE} --ignore-not-found
oc delete service antwort -n ${NAMESPACE} --ignore-not-found
oc delete route antwort -n ${NAMESPACE} --ignore-not-found
oc delete configmap antwort-config -n ${NAMESPACE} --ignore-not-found
oc delete statefulset qdrant -n ${NAMESPACE} --ignore-not-found
oc delete pvc -l app=qdrant -n ${NAMESPACE} --ignore-not-found
```

## Post-Uninstall

Confirm removal completed.
