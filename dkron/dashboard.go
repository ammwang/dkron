package dkron

import (
	"encoding/json"
	"html/template"
	"net/http"

	etcdc "github.com/coreos/go-etcd/etcd"
	"github.com/gorilla/mux"
)

type commonDashboardData struct {
	Version    string
	LeaderName string
	MemberName string
}

func newCommonDashboardData(a *AgentCommand, nodeName string) *commonDashboardData {
	l, _ := a.leaderMember()
	return &commonDashboardData{
		Version:    a.Version,
		LeaderName: l.Name,
		MemberName: nodeName,
	}
}

func (a *AgentCommand) dashboardRoutes(r *mux.Router) {
	r.Path("/dashboard").HandlerFunc(a.dashboardIndexHandler).Methods("GET")
	subui := r.PathPrefix("/dashboard").Subrouter()
	subui.HandleFunc("/jobs", a.dashboardJobsHandler).Methods("GET")
	subui.HandleFunc("/jobs/{job}/executions", a.dashboardExecutionsHandler).Methods("GET")
}

func (a *AgentCommand) dashboardIndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	tmpl := template.Must(template.New("dashboard.html.tmpl").ParseFiles(
		"templates/dashboard.html.tmpl", "templates/index.html.tmpl", "templates/status.html.tmpl"))

	rr := etcdc.NewRawRequest("GET", "../version", nil, nil)
	res, err := a.etcd.Client.SendRequest(rr)
	if err != nil {
		log.Error(err)
	}
	var version struct {
		Etcdserver  string `json:"etcdserver"`
		Etcdcluster string `json:"etcdcluster"`
	}
	json.Unmarshal(res.Body, &version)

	var ss *EtcdServerStats
	rr = etcdc.NewRawRequest("GET", "stats/self", nil, nil)
	res, err = a.etcd.Client.SendRequest(rr)
	if err != nil {
		log.Error(err)
	}
	json.Unmarshal(res.Body, &ss)

	data := struct {
		Common      *commonDashboardData
		EtcdVersion string
		Stats       *EtcdServerStats
		StartTime   string
	}{
		Common:      newCommonDashboardData(a, a.config.NodeName),
		EtcdVersion: version.Etcdserver,
		Stats:       ss,
		StartTime:   ss.LeaderInfo.StartTime.Format("2/Jan/2006 15:05:05"),
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Error(err)
	}
}

func (a *AgentCommand) dashboardJobsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	jobs, _ := a.etcd.GetJobs()

	funcs := template.FuncMap{
		"isSuccess": func(job *Job) bool {
			return job.LastSuccess.After(job.LastError)
		},
	}

	tmpl := template.Must(template.New("dashboard.html.tmpl").Funcs(funcs).ParseFiles(
		"templates/dashboard.html.tmpl", "templates/jobs.html.tmpl", "templates/status.html.tmpl"))

	data := struct {
		Common *commonDashboardData
		Jobs   []*Job
	}{
		Common: newCommonDashboardData(a, a.config.NodeName),
		Jobs:   jobs,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Error(err)
	}
}

func (a *AgentCommand) dashboardExecutionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	vars := mux.Vars(r)
	job := vars["job"]

	execs, _ := a.etcd.GetExecutions(job)

	tmpl := template.Must(template.New("dashboard.html.tmpl").Funcs(template.FuncMap{
		"html": func(value []byte) template.HTML {
			return template.HTML(value)
		},
	}).ParseFiles("templates/dashboard.html.tmpl", "templates/executions.html.tmpl"))

	if len(execs) > 100 {
		execs = execs[len(execs)-100:]
	}

	data := struct {
		Common     *commonDashboardData
		Executions []*Execution
		JobName    string
	}{
		Common:     newCommonDashboardData(a, a.config.NodeName),
		Executions: execs,
		JobName:    job,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Error(err)
	}
}
