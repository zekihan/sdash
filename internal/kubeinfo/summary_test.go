package kubeinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testClient(t *testing.T) *Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[
			{"metadata":{"name":"node-a","labels":{"node-role.kubernetes.io/control-plane":""}},
			 "status":{"conditions":[{"type":"Ready","status":"True"}],"nodeInfo":{"kubeletVersion":"v1.31.0"}}},
			{"metadata":{"name":"node-b","labels":{}},
			 "status":{"conditions":[{"type":"Ready","status":"False"}],"nodeInfo":{"kubeletVersion":"v1.31.0"}}}
		]}`))
	})
	mux.HandleFunc("/api/v1/pods", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[
			{"metadata":{"name":"web-0","namespace":"apps"},
			 "status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":0}],
			  "conditions":[{"type":"Ready","status":"True"}]}},
			{"metadata":{"name":"job-0","namespace":"apps"},
			 "status":{"phase":"Succeeded","containerStatuses":[{"ready":false,"restartCount":0}]}},
			{"metadata":{"name":"bad-0","namespace":"apps"},
			 "status":{"phase":"Running","containerStatuses":[{"ready":false,"restartCount":7}]}},
			{"metadata":{"name":"gated-0","namespace":"apps"},
			 "spec":{"readinessGates":[{"conditionType":"example.com/backend-registered"}]},
			 "status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":0}],
			  "conditions":[{"type":"ContainersReady","status":"True"},
			                {"type":"Ready","status":"False","reason":"ReadinessGatesNotReady"}]}}
		]}`))
	})
	mux.HandleFunc("/apis/apps/v1/deployments", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[
			{"metadata":{"name":"web","namespace":"apps"},"spec":{"replicas":2},"status":{"readyReplicas":2}},
			{"metadata":{"name":"api","namespace":"apps"},"spec":{"replicas":3},"status":{"readyReplicas":1}}
		]}`))
	})
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{},{},{}]}`))
	})
	mux.HandleFunc("/apis/argoproj.io/v1alpha1/namespaces/argocd/applications",
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"items":[
				{"metadata":{"name":"a"},"status":{"sync":{"status":"Synced"},"health":{"status":"Healthy"}}},
				{"metadata":{"name":"b"},"status":{"sync":{"status":"OutOfSync"},"health":{"status":"Degraded"}}}
			]}`))
		})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := new(Client)
	client.httpClient = server.Client()
	client.base = server.URL
	client.argocdNS = "argocd"

	return client
}

func TestSummaryAggregates(t *testing.T) {
	summary, err := testClient(t).Summary(t.Context())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	checkNodesAndPods(t, summary)
	checkDeploymentsAndArgo(t, summary)
}

func checkNodesAndPods(t *testing.T, summary *Summary) {
	t.Helper()

	if !summary.Available || summary.NodesReady != 1 || summary.NodesTotal != 2 {
		t.Errorf(
			"nodes = %d/%d available=%v, want 1/2 true",
			summary.NodesReady,
			summary.NodesTotal,
			summary.Available,
		)
	}

	if summary.Nodes[0].Roles != "control-plane" {
		t.Errorf("node-a roles = %q, want control-plane", summary.Nodes[0].Roles)
	}

	if summary.PodsReady != 2 || summary.PodsTotal != 4 {
		t.Errorf(
			"pods = %d/%d, want 2/4 (succeeded counts ready)",
			summary.PodsReady,
			summary.PodsTotal,
		)
	}

	if len(summary.PodsNotReady) != 2 || summary.PodsNotReady[0].Restarts != 7 {
		t.Fatalf("podsNotReady = %+v, want bad-0 with 7 restarts and gated-0", summary.PodsNotReady)
	}

	gated := summary.PodsNotReady[1]
	if gated.Name != "gated-0" || gated.Ready != 1 || gated.Desired != 1 ||
		gated.Detail != "ReadinessGatesNotReady" {
		t.Errorf(
			"gated pod = %+v, want gated-0 with 1/1 containers and readiness gate reason",
			gated,
		)
	}
}

func checkDeploymentsAndArgo(t *testing.T, summary *Summary) {
	t.Helper()

	if summary.DeploymentsReady != 1 || len(summary.DeploysNotReady) != 1 {
		t.Errorf(
			"deployments ready=%d notReady=%d, want 1/1",
			summary.DeploymentsReady,
			len(summary.DeploysNotReady),
		)
	}

	if summary.Namespaces != 3 {
		t.Errorf("namespaces = %d, want 3", summary.Namespaces)
	}

	if !summary.Argo.Available || summary.Argo.Synced != 1 || summary.Argo.Healthy != 1 ||
		len(summary.Argo.Applications) != 1 {
		t.Errorf("argo = %+v, want 1 synced, 1 healthy, 1 attention", summary.Argo)
	}
}

func TestSummaryFailsWhenCoreAPIDown(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	client := new(Client)
	client.httpClient = server.Client()
	client.base = server.URL
	client.argocdNS = "argocd"

	_, err := client.Summary(t.Context())
	if err == nil {
		t.Fatal("Summary succeeded against a 404 API, want error")
	}
}
