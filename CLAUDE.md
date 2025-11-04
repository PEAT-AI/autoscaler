# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is the Kubernetes Autoscaler repository containing three main autoscaling components:
- **Cluster Autoscaler** - Adjusts cluster size by adding/removing nodes
- **Vertical Pod Autoscaler (VPA)** - Adjusts pod resource requests/limits based on usage
- **Addon Resizer** - Simplified VPA that scales deployments based on node count

Current branch: `cluster-autoscaler-1.31.2-cortex` (main branch: `master`)

## Build and Test Commands

### Cluster Autoscaler

Build:
```bash
cd cluster-autoscaler
make build                    # Build for current arch
make build-arch-amd64        # Build for specific arch
BUILD_TAGS=aws make build    # Build with specific cloud provider
```

Test:
```bash
cd cluster-autoscaler
make test-unit               # Run unit tests with race detection
go test ./...                # Run all tests
go test ./core/...           # Run tests in specific package
go test -v -run TestName ./path/to/package  # Run specific test
```

Build with Docker:
```bash
cd cluster-autoscaler
make docker-builder          # Build the builder container
make build-in-docker         # Build inside Docker
make test-in-docker          # Run tests inside Docker
```

Format and verify:
```bash
cd cluster-autoscaler
make format                  # Format Go code
```

### Vertical Pod Autoscaler

Deploy/test VPA:
```bash
cd vertical-pod-autoscaler
./hack/vpa-up.sh            # Deploy VPA to cluster
./hack/vpa-down.sh          # Remove VPA from cluster
./hack/run-e2e-tests.sh     # Run E2E tests
```

### Repository-wide

Run all verification checks:
```bash
./hack/verify-all.sh        # Run all verify scripts (gofmt, golint, boilerplate, spelling)
./hack/verify-all.sh -v     # Verbose mode
```

Run tests for all Go projects:
```bash
./hack/for-go-proj.sh test   # Tests addon-resizer, VPA, and cluster-autoscaler
./hack/for-go-proj.sh build  # Build all projects
```

## Cluster Autoscaler Architecture

### Core Components

**Main Loop** (`cluster-autoscaler/loop/`)
- Runs `StaticAutoscaler.RunOnce()` periodically (default: every 10 seconds)
- Entry point: `cluster-autoscaler/main.go` (5000+ lines with extensive flag definitions)

**StaticAutoscaler** (`cluster-autoscaler/core/static_autoscaler.go`)
- Main orchestrator implementing the autoscaler control loop
- Manages `AutoscalingContext` - central context object with all configuration, clients, and state
- Coordinates scale-up and scale-down decisions
- Processes nodes and pods through configurable `Processors`

**Scale-Up Flow** (`cluster-autoscaler/core/scaleup/orchestrator/`)
1. Collects unschedulable pods (filtered by age and custom processors)
2. Groups pods with identical scheduling requirements via equivalence classes
3. Filters valid node groups based on resource limits and backoff state
4. Uses `BinpackingEstimator` to simulate placement and determine required node count
5. Applies `Expander` strategy (Priority, Cost, Binpacking, or Balanced) to select best node group
6. Calls `nodeGroup.IncreaseSize(delta)` to provision nodes
7. Tracks scale-up in `ClusterStateRegistry` for monitoring

**Scale-Down Flow** (`cluster-autoscaler/core/scaledown/`)
- `ScaleDownPlanner`: Selects nodes that are underutilized and can be safely removed
- `ScaleDownActuator`: Drains pods and deletes nodes (supports parallel drain mode)
- Uses `RemovalSimulator` and `ClusterSnapshot` to simulate removal and verify pod reschedulability
- Enforces PodDisruptionBudget constraints and grace periods
- Rate-limited by `max-scale-down-parallelism` and cooldown periods

### Key Packages

| Package | Purpose |
|---------|---------|
| `core/` | Main autoscaler implementation and scale-up/down orchestration |
| `clusterstate/` | Tracks node group health, pending requests, and cluster state via `ClusterStateRegistry` |
| `cloudprovider/` | Cloud provider abstraction with 30+ implementations (AWS, Azure, GCP, etc.) |
| `estimator/` | `BinpackingEstimator` simulates pod placement to determine node requirements |
| `simulator/` | `RemovalSimulator` and `ClusterSnapshot` for safe scale-down decisions |
| `processors/` | Pluggable processors for filtering pods, node groups, and scale-down candidates |
| `expander/` | Strategies for selecting which node group to scale up (priority, cost, binpacking) |
| `context/` | `AutoscalingContext` bundles configuration, clients, and strategies |
| `config/` | Configuration parsing and validation |
| `utils/` | Utilities for drain, GPU, labels, node deletion, etc. |

### Cloud Provider Integration

**Cloud Provider Interface** (`cluster-autoscaler/cloudprovider/cloud_provider.go`):
- Defines `CloudProvider` and `NodeGroup` interfaces
- Each provider implements node group discovery, size adjustment, and instance management
- Build tags control which providers are compiled (e.g., `BUILD_TAGS=aws`)

**Builder Pattern** (`cluster-autoscaler/cloudprovider/builder/`):
- Provider-specific builder files: `builder_aws.go`, `builder_azure.go`, etc.
- `builder_all.go` registers all providers for the all-in-one build

Supported providers include: AWS, Azure, GCP, Alicloud, Baiducloud, Hetzner, DigitalOcean, Linode, and 20+ others.

### Important Implementation Details

**Processor Pattern**: Allows customization at multiple phases:
- `PodListProcessor` - filter unschedulable pods
- `NodeGroupListProcessor` - filter available node groups
- `ScaleDownNodeProcessor` - pre-filter scale-down candidates
- `ScaleDownSetProcessor` - final scale-down selection

**Simulator Pattern**: In-memory cluster representation for safe decision-making:
- `ClusterSnapshot` - efficient snapshot of current cluster state
- `PredicateChecker` - uses Kubernetes scheduler framework to validate pod placement
- Enables testing "what-if" scenarios without affecting the real cluster

**Backoff Strategy**: `ClusterStateRegistry` uses exponential backoff for failing node groups to prevent thrashing.

## Vertical Pod Autoscaler Architecture

VPA consists of three components:

**Recommender** (`vertical-pod-autoscaler/pkg/recommender/`)
- Watches pod resource usage via metrics API
- Generates resource recommendations using histogram-based model
- Stores recommendations in VPA CRD status

**Updater** (`vertical-pod-autoscaler/pkg/updater/`)
- Evicts pods that need resource updates
- Respects PodDisruptionBudgets and update policies

**Admission Controller** (`vertical-pod-autoscaler/pkg/admission-controller/`)
- Mutating webhook that applies recommendations to new pods
- Runs on pod creation/update

## Development Guidelines

### Code Organization

The repository follows Kubernetes conventions:
- Code must be checked out under `$GOPATH/src/k8s.io/autoscaler`
- Uses `GO111MODULE=auto` for builds (maintaining compatibility with GOPATH)
- Each component has its own `go.mod` file

### Running a Single Test

```bash
cd cluster-autoscaler
go test -v -run TestScaleUp ./core/scaleup/orchestrator/
go test -v -run TestClusterStateRegistry ./clusterstate/
```

### Testing Cloud Providers

```bash
cd cluster-autoscaler
BUILD_TAGS=aws go test ./cloudprovider/aws/...
BUILD_TAGS=azure go test ./cloudprovider/azure/...
```

### Adding a New Cloud Provider

1. Create `cloudprovider/yourprovider/` directory
2. Implement `CloudProvider` and `NodeGroup` interfaces
3. Add `builder_yourprovider.go` with build tag
4. Register in `builder_all.go`

### Debugging Tips

Main configuration flags are in `cluster-autoscaler/main.go`:
- `--cloud-provider` - which cloud provider to use
- `--nodes` - static node group configuration
- `--max-nodes-total` - cluster-wide node limit
- `--scale-down-enabled` - enable/disable scale-down
- `--scale-down-delay-after-add` - cooldown after scale-up
- `--expander` - strategy for node group selection (priority, cost, binpacking)

Key files for understanding behavior:
- `cluster-autoscaler/core/static_autoscaler.go:RunOnce()` - main loop logic
- `cluster-autoscaler/core/scaleup/orchestrator/orchestrator.go:ScaleUp()` - scale-up logic
- `cluster-autoscaler/core/scaledown/planner/planner.go` - scale-down planning

### Important Annotations

Cluster Autoscaler respects several node/pod annotations:
- `cluster-autoscaler.kubernetes.io/scale-down-disabled` - prevent node scale-down
- `cluster-autoscaler.kubernetes.io/safe-to-evict` - mark pod as evictable

## Contributing

- Code must pass `hack/verify-all.sh` checks
- Tests required for new features
- Follow Kubernetes [Go style guidelines](https://github.com/kubernetes/community/tree/master/contributors/devel)
- PRs require LGTM from at least one maintainer
