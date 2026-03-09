---
description: Remove Antwort (Production)
---

# Uninstall Antwort (Production)

## Prerequisites Check

Verify AWS and ROSA authentication from `<rosa-auth>` context.

## Confirmation

Ask the user to confirm removal. Warn that PostgreSQL data (responses, conversations, files) will be lost if the co-located database is removed.

Offer options:
1. Remove Antwort only (keep PostgreSQL data)
2. Remove everything (Antwort + PostgreSQL)
3. Cancel

## Removal

Delete Antwort resources:

```bash
oc delete deployment antwort -n ${NAMESPACE} --ignore-not-found
oc delete service antwort -n ${NAMESPACE} --ignore-not-found
oc delete route antwort -n ${NAMESPACE} --ignore-not-found
oc delete configmap antwort-config -n ${NAMESPACE} --ignore-not-found
```

If full removal selected, also delete PostgreSQL:

```bash
oc delete statefulset postgres -n ${NAMESPACE} --ignore-not-found
oc delete service postgres -n ${NAMESPACE} --ignore-not-found
oc delete pvc -l app=postgres -n ${NAMESPACE} --ignore-not-found
```

## Post-Uninstall

Confirm removal completed. Note remaining resources if partial removal was selected.
