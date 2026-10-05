# WakeTrail Session: kubernetes-live-test

## Summary

| Metric | Value |
| --- | --- |
| Started | 2026-10-04 17:00:32 |
| Ended | 2026-10-04 17:02:44 |
| Duration | 2m13s |
| Events | 2 |
| Failures | 1 |
| Recoveries | 1 |
| Notes | 0 |

## Timeline

### Kubernetes: STATE CHANGE

&nbsp;&nbsp;&nbsp;&nbsp;**Deployment: `default/hello`**  
&nbsp;&nbsp;&nbsp;&nbsp;17:01:15: **Effects:**  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;**Deployment: `default/hello`**  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`desired      1 → 0`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`updated      1 → 0`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`ready        1 → 0`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`available    1 → 0`  
  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;**Pod: `default/hello-775d79c56b-cm6mk`**  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`Running ready 1/1`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`→ Succeeded/Completed ready 0/1`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`→ Disappeared`  
  
&nbsp;&nbsp;&nbsp;&nbsp;**Command:** `kubectl scale deployment hello --replicas=0`

---

### Kubernetes: RECOVERY

&nbsp;&nbsp;&nbsp;&nbsp;**Deployment: `default/hello`**  
&nbsp;&nbsp;&nbsp;&nbsp;17:01:39: **Effects:**  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;**Deployment: `default/hello`**  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`desired      0 → 1`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`updated      0 → 1`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`ready        0 → 1`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`available    0 → 1`  
  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;**Pod: `default/hello-775d79c56b-x24jr`**  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`Appeared`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`→ Pending/ContainerCreating ready 0/1`  
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;`→ Running ready 1/1`  
  
&nbsp;&nbsp;&nbsp;&nbsp;**Command:** `kubectl scale deployment hello --replicas=1`

---

