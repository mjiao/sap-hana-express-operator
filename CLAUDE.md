# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Git Commit Rules
- Never commit with user "claude"
- Use the existing git configuration (mjiao <manjun.jiao@gmail.com>)
- Do not include Claude Code attribution in commit messages

## Common Development Commands

### Build and Test
- `make build` - Build the operator binary
- `make test` - Run all tests with coverage
- `make fmt` - Format Go code
- `make vet` - Run Go vet
- `make manifests` - Generate CRDs, RBAC, and webhook configurations after API changes

### Local Development
- `make install` - Install CRDs into cluster
- `make run` - Run controller locally (requires CRDs to be installed)
- `make install run` - Install CRDs and run controller in one step

### Docker and Deployment
- `make docker-build IMG=<registry>/operator:tag` - Build container image
- `make docker-push IMG=<registry>/operator:tag` - Push container image
- `make deploy IMG=<registry>/operator:tag` - Deploy operator to cluster
- `make undeploy` - Remove operator from cluster
- `make uninstall` - Remove CRDs from cluster

### Testing Sample Resources
- `kubectl apply -f config/samples/` - Apply sample HanaExpress resources
- Sample requires a secret: `kubectl apply -f config/samples/secret_hxepasswd.yaml`

## Code Architecture

### Project Structure
This is a Kubebuilder v3 Kubernetes operator managing SAP HANA Express database instances.

**Core Components:**
- **API Types** (`api/v1alpha1/hanaexpress_types.go`): Defines the HanaExpress CRD with spec fields for PVC size, credentials, and data persistence settings
- **Controller** (`controllers/hanaexpress_controller.go`): Reconciles HanaExpress resources by creating StatefulSets, Services, and managing PVCs
- **Main** (`main.go`): Sets up manager, registers controllers and schemes

### Key Design Patterns
- **Finalizers**: Used for cleanup operations, especially PVC deletion when `isDataPersisted=false`
- **Status Conditions**: Tracks resource state with "Available", "Progressing", and "Degraded" conditions
- **Owner References**: Ensures proper garbage collection of created resources

### Resource Management
The operator creates and manages:
- **StatefulSet**: Runs SAP HANA Express container with persistent storage
- **Service**: Exposes database ports (39013, 39017, 39041, 59013, 8090)
- **PVC**: Provides persistent storage, conditionally deleted based on `isDataPersisted` setting

### API Validation
- PVC size must match pattern `^\d+Gi$` (e.g., "1Gi", "10Gi")
- Credentials reference Kubernetes secrets with specific key structure
- `isDataPersisted` controls PVC lifecycle behavior