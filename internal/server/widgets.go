package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/zekihan/sdash/config/dashboard"
	"github.com/zekihan/sdash/internal/appwidgets"
	"github.com/zekihan/sdash/internal/crowdsecinfo"
	"github.com/zekihan/sdash/internal/gatusinfo"
	"github.com/zekihan/sdash/internal/kubeinfo"
	"github.com/zekihan/sdash/internal/pbsinfo"
	"github.com/zekihan/sdash/internal/proxmoxinfo"
	"github.com/zekihan/sdash/internal/truenasinfo"
)

const widgetTimeout = 9 * time.Second

func (srv *Server) handleKubernetes(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Kube == nil {
		writeJSON(w, http.StatusOK, unavailableKube())

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), widgetTimeout)
	defer cancel()

	summary, err := srv.deps.Kube.Summary(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "Kubernetes summary failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, unavailableKube())

		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (srv *Server) handleProxmox(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Proxmox == nil {
		writeJSON(w, http.StatusOK, unavailableProxmox())

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), widgetTimeout)
	defer cancel()

	summary, err := srv.deps.Proxmox.Summary(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "Proxmox summary failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, unavailableProxmox())

		return
	}

	summary.WakeHosts = srv.wakeHostNames()
	writeJSON(w, http.StatusOK, summary)
}

func (srv *Server) wakeHostNames() []string {
	names := make([]string, 0, len(srv.deps.Config.Actions.WakeOnLan))
	for _, host := range srv.deps.Config.Actions.WakeOnLan {
		names = append(names, host.Name)
	}

	return names
}

func unavailableProxmox() *proxmoxinfo.Summary {
	summary := new(proxmoxinfo.Summary)
	summary.Nodes = []proxmoxinfo.Node{}
	summary.Storage = []proxmoxinfo.Storage{}
	summary.Stopped = []proxmoxinfo.Guest{}

	return summary
}

func (srv *Server) handleTrueNAS(w http.ResponseWriter, r *http.Request) {
	if srv.deps.TrueNAS == nil {
		writeJSON(w, http.StatusOK, unavailableTrueNAS())

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), widgetTimeout)
	defer cancel()

	summary, err := srv.deps.TrueNAS.Summary(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "TrueNAS summary failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, unavailableTrueNAS())

		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func unavailableTrueNAS() *truenasinfo.Summary {
	summary := new(truenasinfo.Summary)
	summary.Pools = []truenasinfo.Pool{}

	return summary
}

func unavailableKube() *kubeinfo.Summary {
	summary := new(kubeinfo.Summary)
	summary.Nodes = []kubeinfo.Node{}
	summary.PodsNotReady = []kubeinfo.Workload{}
	summary.DeploysNotReady = []kubeinfo.Workload{}
	summary.Argo.Applications = []kubeinfo.ArgoApplication{}

	return summary
}

func (srv *Server) handlePBS(w http.ResponseWriter, r *http.Request) {
	if srv.deps.PBS == nil {
		writeJSON(w, http.StatusOK, unavailablePBS())

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), widgetTimeout)
	defer cancel()

	summary, err := srv.deps.PBS.Summary(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "PBS summary failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, unavailablePBS())

		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func unavailablePBS() *pbsinfo.Summary {
	summary := new(pbsinfo.Summary)
	summary.Datastores = []pbsinfo.Datastore{}

	return summary
}

func (srv *Server) handleCrowdSec(w http.ResponseWriter, r *http.Request) {
	if srv.deps.CrowdSec == nil {
		writeJSON(w, http.StatusOK, unavailableCrowdSec())

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), widgetTimeout)
	defer cancel()

	summary, err := srv.deps.CrowdSec.Summary(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "CrowdSec summary failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, unavailableCrowdSec())

		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func unavailableCrowdSec() *crowdsecinfo.Summary {
	summary := new(crowdsecinfo.Summary)
	summary.Groups = []crowdsecinfo.DecisionGroup{}

	return summary
}

func (srv *Server) handleGatus(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Gatus == nil {
		writeJSON(w, http.StatusOK, unavailableGatus())

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), widgetTimeout)
	defer cancel()

	summary, err := srv.deps.Gatus.Summary(ctx)
	if err != nil {
		slog.WarnContext(r.Context(), "Gatus summary failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, unavailableGatus())

		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func unavailableGatus() *gatusinfo.Summary {
	summary := new(gatusinfo.Summary)
	summary.Endpoints = []gatusinfo.Endpoint{}

	return summary
}

func (srv *Server) handleServiceWidgets(w http.ResponseWriter, r *http.Request) {
	if srv.deps.Widgets == nil || !srv.deps.Widgets.Enabled() {
		writeJSON(w, http.StatusOK, map[string]*appwidgets.Widget{})

		return
	}

	cfg := dashboard.GetConfigWithContext(r.Context())
	writeJSON(w, http.StatusOK, srv.deps.Widgets.ForServices(r.Context(), cfg.ServicesList))
}
