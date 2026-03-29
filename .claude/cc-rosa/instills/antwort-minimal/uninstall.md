---
description: Remove Antwort (Minimal)
---

# Uninstall Antwort (Minimal)

## Prerequisites Check

Check the `<rosa-auth>` context in the system reminder.

**If AWS Valid is False:**
- Inform the user they need to authenticate with AWS first

## Confirmation

Ask the user to confirm removal. Explain that in-memory storage means all responses will be lost (there is no persistent data to preserve).

## Removal

Delete all Antwort resources in the namespace:

```bash
oc delete deployment antwort -n ${NAMESPACE} --ignore-not-found
oc delete service antwort -n ${NAMESPACE} --ignore-not-found
oc delete route antwort -n ${NAMESPACE} --ignore-not-found
oc delete configmap antwort-config -n ${NAMESPACE} --ignore-not-found
```

## Cleanup

Ask if the user wants to delete the namespace entirely:

```bash
oc delete namespace ${NAMESPACE} --ignore-not-found
```

## Post-Uninstall

Confirm removal completed. Note that the model InferenceService is not affected.
