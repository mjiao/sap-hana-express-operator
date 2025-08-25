# SAP HANA Express Operator

A Kubernetes Operator for managing the lifecycle of SAP HANA Express Edition deployments on Kubernetes platforms.

## Overview

SAP HANA Express Edition is a streamlined version of SAP HANA for development, testing, and learning purposes. This operator automates the deployment, configuration, and management of HANA Express instances in Kubernetes environments.

**Key Features:**
- Automated StatefulSet and Service provisioning
- Persistent volume management with configurable retention policies
- Secure credential management through Kubernetes secrets
- Multi-port service exposure for HANA database access
- Proper security context and permission handling
- Finalizer-based cleanup operations

## Architecture

The operator manages:
- **StatefulSet**: Runs HANA Express containers with persistent storage
- **Service**: Exposes database ports (39013, 39017, 39041, 59013, 8090) 
- **PersistentVolumeClaims**: Handles data persistence with optional cleanup
- **Security**: Non-root containers with proper user/group settings (12000:79)

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured to access your cluster
- Container registry access for custom images (if building locally)
- Sufficient cluster resources for HANA Express workloads

## Quick Start

### 1. Install the Operator

```bash
# Install CRDs
make install

# Deploy the operator to your cluster
make deploy IMG=quay.io/redhat-sap-cop/sap-hana-express-operator:latest
```

### 2. Create Required Secrets

**Option A: Simple Plain Text Password (Recommended)**
```bash
# Create secret with plain text password (simpler)
kubectl create secret generic hana-password \
  --from-literal=master-password='YourSecurePassword123'
```

**Option B: JSON Format (Advanced)**
```bash
# Create secret with JSON credentials (for complex setups)
kubectl create secret generic hxepasswd \
  --from-literal=hxepasswd.json='{"master_password": "YourSecurePassword123"}'
```

Or apply the provided samples:
```bash
# Plain text format
kubectl apply -f config/samples/secret_hana_plaintext.yaml

# JSON format
kubectl apply -f config/samples/secret_hxepasswd.yaml
```

### 3. Deploy HANA Express Instance

```bash
# Create a HanaExpress instance
kubectl apply -f config/samples/db_v1alpha1_hanaexpress.yaml
```

### 4. Verify Deployment

```bash
# Check the HanaExpress status
kubectl get hanaexpress

# Check StatefulSet and Pods
kubectl get statefulsets,pods

# Check Service
kubectl get services
```

## Configuration

### HanaExpress Custom Resource

```yaml
apiVersion: db.sap-redhat.io/v1alpha1
kind: HanaExpress
metadata:
  name: my-hana-instance
spec:
  # Required: PVC size (must match pattern ^\d+Gi$)
  pvcSize: "50Gi"
  
  # Required: Reference to secret containing HANA credentials
  credential:
    secretKeyRef:
      name: hana-password     # Secret name
      key: master-password    # Key within secret
    format: plain             # Can be 'plain' or 'json'
  
  # Optional: Whether to preserve data when CR is deleted (default: false)
  isDataPersisted: true
```

### Configuration Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pvcSize` | string | Yes | Persistent volume size (e.g., "1Gi", "50Gi") |
| `credential.secretKeyRef.name` | string | Yes | Name of Kubernetes secret containing credentials |
| `credential.secretKeyRef.key` | string | Yes | Key within the secret containing password |
| `credential.format` | string | No | Format of credential data: "plain" or "json" (default: "plain") |
| `isDataPersisted` | boolean | No | Preserve PVC when HanaExpress is deleted (default: false) |

### Environment Variables

The operator requires the following environment variable:

- `HANAEXPRESS_IMAGE`: Container image for SAP HANA Express Edition

## Usage Examples

### Development Instance (Simple Plain Text)
```yaml
apiVersion: db.sap-redhat.io/v1alpha1
kind: HanaExpress
metadata:
  name: hana-dev
spec:
  pvcSize: "10Gi"
  credential:
    secretKeyRef:
      name: hana-password
      key: master-password
    format: plain  # Simple plain text password
  isDataPersisted: false
```

### Production Instance (JSON Format)
```yaml
apiVersion: db.sap-redhat.io/v1alpha1
kind: HanaExpress
metadata:
  name: hana-prod
spec:
  pvcSize: "500Gi"
  credential:
    secretKeyRef:
      name: hana-prod-credentials
      key: credentials.json
    format: json  # JSON format for complex credentials
  isDataPersisted: true
```

## Accessing HANA Express

Once deployed, connect to HANA Express using:
- **Host**: Service name (e.g., `hana-dev.default.svc.cluster.local`)
- **Ports**: 39013 (SQL), 39017 (SQL), 39041 (XSA), 8090 (Cockpit)
- **Credentials**: From the configured secret

### Port Forwarding for Local Access
```bash
# Forward SQL port for local connections
kubectl port-forward service/hana-dev 39017:39017

# Connect using HANA Studio or SQL client
# Host: localhost:39017
```

## Troubleshooting

### Common Issues

**Pod stuck in Pending state:**
- Check if PVC can be provisioned: `kubectl get pvc`
- Verify storage class exists and has available capacity
- Check node resources: `kubectl describe nodes`

**Pod fails with permission errors:**
- Ensure the HANA Express image supports non-root execution
- Verify the init container completed successfully
- Check security context settings in the StatefulSet

**Secret not found errors:**
- Verify secret exists: `kubectl get secret hxepasswd`
- Check secret has the correct key specified in credential.key
- Ensure secret is in the same namespace as HanaExpress resource

**HANA startup issues:**
- Check pod logs: `kubectl logs <pod-name> -c hana-express`
- Verify password format in secret matches HANA requirements
- Check available memory and CPU resources

### Monitoring

Monitor HanaExpress instances:

```bash
# Check HanaExpress status
kubectl get hanaexpress -o wide

# View operator logs
kubectl logs -n sap-hana-express-operator-system deployment/sap-hana-express-operator-controller-manager

# Check StatefulSet status
kubectl describe statefulset <hanaexpress-name>

# Monitor pod resources
kubectl top pods
```

### Cleanup

```bash
# Delete HanaExpress instance (preserves data if isDataPersisted: true)
kubectl delete hanaexpress <instance-name>

# Uninstall operator
make undeploy

# Remove CRDs
make uninstall

# Manual PVC cleanup (if needed)
kubectl delete pvc data-<instance-name>-0
```

## Development

### Local Development

1. Install dependencies:
```bash
make install
```

2. Run controller locally:
```bash
make run
```

3. Run tests:
```bash
make test
```

### Building Custom Images

```bash
# Build and push operator image
make docker-build docker-push IMG=<your-registry>/sap-hana-express-operator:tag

# Deploy with custom image
make deploy IMG=<your-registry>/sap-hana-express-operator:tag
```

### API Modifications

When modifying the API (api/v1alpha1/hanaexpress_types.go):

```bash
# Regenerate manifests and code
make manifests generate

# Update CRDs in cluster
make install
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite: `make test`
6. Submit a pull request

For more information, see the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html).

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

