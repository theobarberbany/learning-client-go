## Test WR Deployment method using client-go

Proof of concept using client go's `remotecommand` to copy  binary to an init container, before running the copied binary as the main process for the pod.

Adds it's own namespace.


