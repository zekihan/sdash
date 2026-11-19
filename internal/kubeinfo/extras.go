package kubeinfo

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"
)

const percentScaleVal = 100.0

const (
	certWarnDays     = 14
	topListSize      = 5
	hoursPerDay      = 24
	millisPerCore    = 1000.0
	nanosPerMilliCPU = 1_000_000.0
)

// Certificate is a cert-manager certificate that is expiring or not ready.
type Certificate struct {
	NotAfter  time.Time `json:"notAfter"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	DaysLeft  int       `json:"daysLeft"`
	Ready     bool      `json:"ready"`
}

// LonghornSummary aggregates Longhorn volume robustness.
type LonghornSummary struct {
	Attention []Workload `json:"attention"`
	Healthy   int        `json:"healthy"`
	Degraded  int        `json:"degraded"`
	Total     int        `json:"total"`
	Available bool       `json:"available"`
}

// PodUsage is one pod's current resource usage from metrics-server.
type PodUsage struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	CPUMillis int64  `json:"cpuMillis"`
	MemBytes  uint64 `json:"memBytes"`
}

// NamespaceUsage aggregates pod metrics per namespace.
type NamespaceUsage struct {
	Name      string `json:"name"`
	CPUMillis int64  `json:"cpuMillis"`
	MemBytes  uint64 `json:"memBytes"`
	Pods      int    `json:"pods"`
}

// MetricsSummary holds metrics-server derived views. Node usage lives on
// the Node entries of the parent Summary.
type MetricsSummary struct {
	TopPods    []PodUsage       `json:"topPods"`
	Namespaces []NamespaceUsage `json:"namespaces"`
	Available  bool             `json:"available"`
}

type certificateList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			NotAfter   time.Time `json:"notAfter"`
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

type longhornVolumeList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			State      string `json:"state"`
			Robustness string `json:"robustness"`
		} `json:"status"`
	} `json:"items"`
}

type nodeMetricsList struct {
	Items []struct {
		Usage    map[string]string `json:"usage"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

type podMetricsList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Containers []struct {
			Usage map[string]string `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

type countList struct {
	Items []struct{} `json:"items"`
}

func (client *Client) fetchCertificates(ctx context.Context, summary *Summary) error {
	list := new(certificateList)

	err := client.getJSON(ctx, "/apis/cert-manager.io/v1/certificates?limit=500", list)
	if err != nil {
		return err
	}

	summary.CertificatesAvailable = true
	summary.CertificatesTotal = len(list.Items)

	for _, item := range list.Items {
		ready := false

		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}

		if ready {
			summary.CertificatesReady++
		}

		daysLeft := int(time.Until(item.Status.NotAfter).Hours() / hoursPerDay)
		if ready && daysLeft > certWarnDays {
			continue
		}

		certificate := new(Certificate)
		certificate.Namespace = item.Metadata.Namespace
		certificate.Name = item.Metadata.Name
		certificate.NotAfter = item.Status.NotAfter
		certificate.DaysLeft = daysLeft
		certificate.Ready = ready
		summary.Certificates = append(summary.Certificates, *certificate)
	}

	sort.Slice(summary.Certificates, func(first, second int) bool {
		return summary.Certificates[first].DaysLeft < summary.Certificates[second].DaysLeft
	})

	return nil
}

func (client *Client) fetchLonghorn(ctx context.Context, summary *Summary) error {
	list := new(longhornVolumeList)

	err := client.getJSON(
		ctx,
		"/apis/longhorn.io/v1beta2/namespaces/longhorn/volumes?limit=500",
		list,
	)
	// A Kubernetes list can be empty even when the requested namespace does not exist.
	if err != nil || len(list.Items) == 0 {
		fallback := new(longhornVolumeList)

		fallbackErr := client.getJSON(
			ctx,
			"/apis/longhorn.io/v1beta2/namespaces/longhorn-system/volumes?limit=500",
			fallback,
		)
		if fallbackErr == nil {
			list = fallback
		} else if err != nil {
			return fallbackErr
		}
	}

	longhorn := new(LonghornSummary)
	longhorn.Available = true
	longhorn.Attention = []Workload{}

	for _, item := range list.Items {
		longhorn.Total++

		switch item.Status.Robustness {
		case "healthy":
			longhorn.Healthy++
		case "degraded":
			longhorn.Degraded++
		}

		if item.Status.Robustness == "healthy" {
			continue
		}

		workload := new(Workload)
		workload.Name = item.Metadata.Name
		workload.Detail = item.Status.State + "/" + item.Status.Robustness
		longhorn.Attention = append(longhorn.Attention, *workload)
	}

	if len(longhorn.Attention) > maxNotReadyListed {
		longhorn.Attention = longhorn.Attention[:maxNotReadyListed]
	}

	summary.Longhorn = *longhorn

	return nil
}

func (client *Client) fetchMetrics(ctx context.Context, summary *Summary) error {
	nodes := new(nodeMetricsList)

	err := client.getJSON(ctx, "/apis/metrics.k8s.io/v1beta1/nodes", nodes)
	if err != nil {
		return err
	}

	usageByNode := make(map[string]map[string]string, len(nodes.Items))
	for _, item := range nodes.Items {
		usageByNode[item.Metadata.Name] = item.Usage
	}

	for index := range summary.Nodes {
		usage, ok := usageByNode[summary.Nodes[index].Name]
		if !ok {
			continue
		}

		summary.Nodes[index].CPUMillis = parseCPUMillis(usage["cpu"])
		summary.Nodes[index].MemUsedBytes = parseMemBytes(usage["memory"])

		if summary.Nodes[index].AllocCPUMillis > 0 {
			summary.Nodes[index].CPUPercent =
				float64(
					summary.Nodes[index].CPUMillis,
				) / float64(
					summary.Nodes[index].AllocCPUMillis,
				) * percentScaleVal
		}

		if summary.Nodes[index].AllocMemBytes > 0 {
			summary.Nodes[index].MemPercent =
				float64(
					summary.Nodes[index].MemUsedBytes,
				) / float64(
					summary.Nodes[index].AllocMemBytes,
				) * percentScaleVal
		}
	}

	pods := new(podMetricsList)

	err = client.getJSON(ctx, "/apis/metrics.k8s.io/v1beta1/pods?limit=500", pods)
	if err != nil {
		return err
	}

	metrics := new(MetricsSummary)
	metrics.Available = true
	metrics.TopPods = []PodUsage{}
	metrics.Namespaces = []NamespaceUsage{}

	byNamespace := make(map[string]*NamespaceUsage)
	allPods := make([]PodUsage, 0, len(pods.Items))

	for _, item := range pods.Items {
		usage := new(PodUsage)
		usage.Namespace = item.Metadata.Namespace
		usage.Name = item.Metadata.Name

		for _, container := range item.Containers {
			usage.CPUMillis += parseCPUMillis(container.Usage["cpu"])
			usage.MemBytes += parseMemBytes(container.Usage["memory"])
		}

		allPods = append(allPods, *usage)

		nsUsage, ok := byNamespace[usage.Namespace]
		if !ok {
			nsUsage = new(NamespaceUsage)
			nsUsage.Name = usage.Namespace
			byNamespace[usage.Namespace] = nsUsage
		}

		nsUsage.CPUMillis += usage.CPUMillis
		nsUsage.MemBytes += usage.MemBytes
		nsUsage.Pods++
	}

	sort.Slice(allPods, func(first, second int) bool {
		return allPods[first].CPUMillis > allPods[second].CPUMillis
	})

	if len(allPods) > topListSize {
		allPods = allPods[:topListSize]
	}

	metrics.TopPods = allPods

	for _, nsUsage := range byNamespace {
		metrics.Namespaces = append(metrics.Namespaces, *nsUsage)
	}

	sort.Slice(metrics.Namespaces, func(first, second int) bool {
		return metrics.Namespaces[first].CPUMillis > metrics.Namespaces[second].CPUMillis
	})

	if len(metrics.Namespaces) > topListSize {
		metrics.Namespaces = metrics.Namespaces[:topListSize]
	}

	summary.Metrics = *metrics

	return nil
}

func (client *Client) fetchIngresses(ctx context.Context, summary *Summary) error {
	ingresses := new(countList)

	err := client.getJSON(ctx, "/apis/networking.k8s.io/v1/ingresses?limit=500", ingresses)
	if err != nil {
		return err
	}

	summary.Ingresses = len(ingresses.Items)

	routes := new(countList)

	err = client.getJSON(ctx, "/apis/traefik.io/v1alpha1/ingressroutes?limit=500", routes)
	if err == nil {
		summary.IngressRoutes = len(routes.Items)
	}

	return nil
}

// parseCPUMillis converts a Kubernetes CPU quantity to millicores.
func parseCPUMillis(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	if before, ok := strings.CutSuffix(raw, "n"); ok {
		value, err := strconv.ParseFloat(before, 64)
		if err != nil {
			return 0
		}

		return int64(value / nanosPerMilliCPU)
	}

	if before, ok := strings.CutSuffix(raw, "u"); ok {
		value, err := strconv.ParseFloat(before, 64)
		if err != nil {
			return 0
		}

		return int64(value / millisPerCore)
	}

	if before, ok := strings.CutSuffix(raw, "m"); ok {
		value, err := strconv.ParseInt(before, 10, 64)
		if err != nil {
			return 0
		}

		return value
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}

	return int64(value * millisPerCore)
}

// parseMemBytes converts a Kubernetes memory quantity to bytes.
func parseMemBytes(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	multipliers := []struct {
		suffix string
		factor float64
	}{
		{"Ki", 1024},
		{"Mi", 1024 * 1024},
		{"Gi", 1024 * 1024 * 1024},
		{"Ti", 1024 * 1024 * 1024 * 1024},
		{"K", 1000},
		{"M", 1000 * 1000},
		{"G", 1000 * 1000 * 1000},
		{"T", 1000 * 1000 * 1000 * 1000},
	}

	for _, multiplier := range multipliers {
		if before, ok := strings.CutSuffix(raw, multiplier.suffix); ok {
			value, err := strconv.ParseFloat(before, 64)
			if err != nil {
				return 0
			}

			return uint64(value * multiplier.factor)
		}
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 {
		return 0
	}

	return uint64(value)
}
