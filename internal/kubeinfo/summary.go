package kubeinfo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

const maxNotReadyListed = 24

// Node is one cluster node in the summary.
type Node struct {
	Name           string  `json:"name"`
	Roles          string  `json:"roles"`
	Version        string  `json:"version"`
	CPUPercent     float64 `json:"cpuPercent"`
	MemPercent     float64 `json:"memPercent"`
	CPUMillis      int64   `json:"cpuMillis"`
	AllocCPUMillis int64   `json:"allocCpuMillis"`
	MemUsedBytes   uint64  `json:"memUsedBytes"`
	AllocMemBytes  uint64  `json:"allocMemBytes"`
	Ready          bool    `json:"ready"`
	RebootPending  bool    `json:"rebootPending"`
}

// Workload is a not-ready pod or deployment worth surfacing.
type Workload struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Detail    string `json:"detail"`
	Ready     int    `json:"ready"`
	Desired   int    `json:"desired"`
	Restarts  int    `json:"restarts,omitempty"`
}

// ArgoApplication is an unhealthy or out-of-sync Argo CD application.
type ArgoApplication struct {
	Name   string `json:"name"`
	Sync   string `json:"sync"`
	Health string `json:"health"`
}

// ArgoSummary aggregates Argo CD application states.
type ArgoSummary struct {
	Applications []ArgoApplication `json:"attention"`
	Total        int               `json:"total"`
	Synced       int               `json:"synced"`
	Healthy      int               `json:"healthy"`
	Available    bool              `json:"available"`
}

// Summary is the dashboard-facing cluster overview.
type Summary struct {
	Nodes                 []Node          `json:"nodes"`
	PodsNotReady          []Workload      `json:"podsNotReady"`
	DeploysNotReady       []Workload      `json:"deploymentsNotReady"`
	Certificates          []Certificate   `json:"certificates"`
	RebootRequired        []string        `json:"rebootRequired"`
	Metrics               MetricsSummary  `json:"metrics"`
	Argo                  ArgoSummary     `json:"argo"`
	Longhorn              LonghornSummary `json:"longhorn"`
	PodsReady             int             `json:"podsReady"`
	NodesTotal            int             `json:"nodesTotal"`
	NodesReady            int             `json:"nodesReady"`
	PodsTotal             int             `json:"podsTotal"`
	DeploymentsReady      int             `json:"deploymentsReady"`
	DeploymentsTotal      int             `json:"deploymentsTotal"`
	Namespaces            int             `json:"namespaces"`
	CertificatesTotal     int             `json:"certificatesTotal"`
	CertificatesReady     int             `json:"certificatesReady"`
	Ingresses             int             `json:"ingresses"`
	IngressRoutes         int             `json:"ingressRoutes"`
	CertificatesAvailable bool            `json:"certificatesAvailable"`
	Available             bool            `json:"available"`
}

type nodeList struct {
	Items []struct {
		Metadata struct {
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
			Name        string            `json:"name"`
		} `json:"metadata"`
		Status struct {
			Allocatable map[string]string `json:"allocatable"`
			NodeInfo    struct {
				KubeletVersion string `json:"kubeletVersion"`
			} `json:"nodeInfo"`
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type podList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			Phase      string `json:"phase"`
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"conditions"`
			ContainerStatuses []struct {
				Ready        bool `json:"ready"`
				RestartCount int  `json:"restartCount"`
			} `json:"containerStatuses"`
		} `json:"status"`
	} `json:"items"`
}

type deploymentList struct {
	Items []struct {
		Spec struct {
			Replicas *int `json:"replicas"`
		} `json:"spec"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			ReadyReplicas int `json:"readyReplicas"`
		} `json:"status"`
	} `json:"items"`
}

type namespaceList struct {
	Items []struct{} `json:"items"`
}

type argoList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Sync struct {
				Status string `json:"status"`
			} `json:"sync"`
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
		} `json:"status"`
	} `json:"items"`
}

// Summary fetches nodes, pods, deployments, namespaces and Argo CD
// applications concurrently and aggregates them. Argo failures degrade to
// unavailable (the CRD may not exist); core failures fail the summary.
func (client *Client) Summary(ctx context.Context) (*Summary, error) {
	summary := new(Summary)
	summary.Nodes = []Node{}
	summary.PodsNotReady = []Workload{}
	summary.DeploysNotReady = []Workload{}
	summary.Certificates = []Certificate{}
	summary.RebootRequired = []string{}
	summary.Argo.Applications = []ArgoApplication{}
	summary.Longhorn.Attention = []Workload{}
	summary.Metrics.TopPods = []PodUsage{}
	summary.Metrics.Namespaces = []NamespaceUsage{}

	var (
		waitGroup sync.WaitGroup
		errs      []error
		errsMu    sync.Mutex
	)

	collect := func(fetch func() error, fatal bool) {
		waitGroup.Go(func() {
			err := fetch()
			if err != nil && fatal {
				errsMu.Lock()

				errs = append(errs, err)
				errsMu.Unlock()
			}
		})
	}

	collect(func() error { return client.fetchNodes(ctx, summary) }, true)
	collect(func() error { return client.fetchPods(ctx, summary) }, true)
	collect(func() error { return client.fetchDeployments(ctx, summary) }, true)
	collect(func() error { return client.fetchNamespaces(ctx, summary) }, false)
	collect(func() error { return client.fetchArgo(ctx, summary) }, false)
	collect(func() error { return client.fetchCertificates(ctx, summary) }, false)
	collect(func() error { return client.fetchLonghorn(ctx, summary) }, false)
	collect(func() error { return client.fetchIngresses(ctx, summary) }, false)

	waitGroup.Wait()

	// Metrics annotate the nodes fetched above, so they run afterwards.
	metricsErr := client.fetchMetrics(ctx, summary)
	if metricsErr != nil {
		slog.DebugContext(ctx, "Metrics fetch failed", slog.Any("error", metricsErr))
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	summary.Available = true

	return summary, nil
}

func (client *Client) fetchNodes(ctx context.Context, summary *Summary) error {
	list := new(nodeList)

	err := client.getJSON(ctx, "/api/v1/nodes", list)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	nodes := make([]Node, 0, len(list.Items))
	readyCount := 0
	rebootRequired := make([]string, 0)

	for _, item := range list.Items {
		node := new(Node)
		node.Name = item.Metadata.Name
		node.Version = item.Status.NodeInfo.KubeletVersion
		node.Roles = nodeRoles(item.Metadata.Labels)
		node.AllocCPUMillis = parseCPUMillis(item.Status.Allocatable["cpu"])
		node.AllocMemBytes = parseMemBytes(item.Status.Allocatable["memory"])

		for annotation := range item.Metadata.Annotations {
			if strings.Contains(annotation, "kured") {
				node.RebootPending = true
			}
		}

		if node.RebootPending {
			rebootRequired = append(rebootRequired, node.Name)
		}

		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				node.Ready = true
			}
		}

		if node.Ready {
			readyCount++
		}

		nodes = append(nodes, *node)
	}

	sort.Strings(rebootRequired)
	summary.RebootRequired = rebootRequired

	sort.Slice(
		nodes,
		func(first, second int) bool { return nodes[first].Name < nodes[second].Name },
	)

	summary.Nodes = nodes
	summary.NodesReady = readyCount
	summary.NodesTotal = len(nodes)

	return nil
}

func nodeRoles(labels map[string]string) string {
	roles := make([]string, 0)

	for label := range labels {
		role, found := strings.CutPrefix(label, "node-role.kubernetes.io/")
		if found && role != "" {
			roles = append(roles, role)
		}
	}

	sort.Strings(roles)

	if len(roles) == 0 {
		return "worker"
	}

	return strings.Join(roles, ",")
}

func (client *Client) fetchPods(ctx context.Context, summary *Summary) error {
	list := new(podList)

	err := client.getJSON(ctx, "/api/v1/pods", list)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	readyCount := 0
	notReady := make([]Workload, 0)

	for _, item := range list.Items {
		if item.Status.Phase == "Succeeded" {
			readyCount++

			continue
		}

		detail := item.Status.Phase
		podReady := false

		if item.Status.Phase == "Running" {
			for _, condition := range item.Status.Conditions {
				if condition.Type != "Ready" {
					continue
				}

				podReady = condition.Status == "True"
				if condition.Reason != "" {
					detail = condition.Reason
				}

				break
			}
		}

		if podReady {
			readyCount++

			continue
		}

		ready := 0
		restarts := 0

		for _, container := range item.Status.ContainerStatuses {
			if container.Ready {
				ready++
			}

			restarts += container.RestartCount
		}

		workload := new(Workload)
		workload.Namespace = item.Metadata.Namespace
		workload.Name = item.Metadata.Name
		workload.Detail = detail
		workload.Ready = ready
		workload.Desired = len(item.Status.ContainerStatuses)
		workload.Restarts = restarts
		notReady = append(notReady, *workload)
	}

	sortWorkloads(notReady)

	summary.PodsReady = readyCount
	summary.PodsTotal = len(list.Items)
	summary.PodsNotReady = capWorkloads(notReady)

	return nil
}

func (client *Client) fetchDeployments(ctx context.Context, summary *Summary) error {
	list := new(deploymentList)

	err := client.getJSON(ctx, "/apis/apps/v1/deployments", list)
	if err != nil {
		return fmt.Errorf("list deployments: %w", err)
	}

	readyCount := 0
	notReady := make([]Workload, 0)

	for _, item := range list.Items {
		desired := 1
		if item.Spec.Replicas != nil {
			desired = *item.Spec.Replicas
		}

		if item.Status.ReadyReplicas >= desired {
			readyCount++

			continue
		}

		workload := new(Workload)
		workload.Namespace = item.Metadata.Namespace
		workload.Name = item.Metadata.Name
		workload.Ready = item.Status.ReadyReplicas
		workload.Desired = desired
		notReady = append(notReady, *workload)
	}

	sortWorkloads(notReady)

	summary.DeploymentsReady = readyCount
	summary.DeploymentsTotal = len(list.Items)
	summary.DeploysNotReady = capWorkloads(notReady)

	return nil
}

func (client *Client) fetchNamespaces(ctx context.Context, summary *Summary) error {
	list := new(namespaceList)

	err := client.getJSON(ctx, "/api/v1/namespaces?limit=500", list)
	if err != nil {
		return err
	}

	summary.Namespaces = len(list.Items)

	return nil
}

func (client *Client) fetchArgo(ctx context.Context, summary *Summary) error {
	list := new(argoList)

	err := client.getJSON(
		ctx,
		"/apis/argoproj.io/v1alpha1/namespaces/"+client.argocdNS+"/applications?limit=500",
		list,
	)
	if err != nil {
		return err
	}

	argo := new(ArgoSummary)
	argo.Available = true
	argo.Applications = []ArgoApplication{}

	for _, item := range list.Items {
		argo.Total++

		if item.Status.Sync.Status == "Synced" {
			argo.Synced++
		}

		if item.Status.Health.Status == "Healthy" {
			argo.Healthy++
		}

		if item.Status.Sync.Status != "Synced" || item.Status.Health.Status != "Healthy" {
			application := new(ArgoApplication)
			application.Name = item.Metadata.Name
			application.Sync = item.Status.Sync.Status
			application.Health = item.Status.Health.Status
			argo.Applications = append(argo.Applications, *application)
		}
	}

	sort.Slice(argo.Applications, func(first, second int) bool {
		return argo.Applications[first].Name < argo.Applications[second].Name
	})

	if len(argo.Applications) > maxNotReadyListed {
		argo.Applications = argo.Applications[:maxNotReadyListed]
	}

	summary.Argo = *argo

	return nil
}

func sortWorkloads(workloads []Workload) {
	sort.Slice(workloads, func(first, second int) bool {
		if workloads[first].Namespace != workloads[second].Namespace {
			return workloads[first].Namespace < workloads[second].Namespace
		}

		return workloads[first].Name < workloads[second].Name
	})
}

func capWorkloads(workloads []Workload) []Workload {
	if len(workloads) > maxNotReadyListed {
		return workloads[:maxNotReadyListed]
	}

	return workloads
}
