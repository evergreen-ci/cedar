package rest

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/units"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/gimlet"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

////////////////////////////////////////////////////////////////////////
//
// GET /status

type StatusResponse struct {
	Revision     string           `json:"revision"`
	QueueStats   amboy.QueueStats `json:"queue,omitempty"`
	QueueRunning bool             `json:"running"`
	RPCInfo      []string         `json:"rpc_service"`
}

// statusHandler processes the GET request for
func (s *Service) statusHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.Environment.Context()
	defer cancel()

	resp := &StatusResponse{
		Revision: cedar.BuildRevision,
		RPCInfo:  s.RPCServers,
	}

	if s.queue != nil {
		resp.QueueRunning = s.queue.Started()
		resp.QueueStats = s.queue.Stats(ctx)
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /status/events/{level}?limit=<int>

type SystemEventsResponse struct {
	Level  string         `json:"level,omitempty"`
	Total  int            `json:"total,omitempty"`
	Count  int            `json:"count,omitempty"`
	Events []*model.Event `json:"events"`
	Err    string         `json:"error"`
}

func (s *Service) getSystemEvents(w http.ResponseWriter, r *http.Request) {
	l := gimlet.GetVars(r)["level"]
	resp := &SystemEventsResponse{}

	if l == "" {
		resp.Err = "no level specified"
		gimlet.WriteJSONError(w, resp)
		return
	}

	if !level.FromString(l).IsValid() {
		resp.Err = fmt.Sprintf("%s is not a valid level", l)
	}
	resp.Level = l

	limitArg := r.URL.Query()["limit"][0]
	limit, err := strconv.Atoi(limitArg)
	if err != nil {
		resp.Err = fmt.Sprintf("%s is not a valid limit [%s]", limitArg, err.Error())
		gimlet.WriteJSONError(w, resp)
		return
	}

	e := &model.Events{}
	err = e.FindLevel(l, limit)
	if err != nil {
		resp.Err = "problem running query for events"
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.Events = e.Slice()
	resp.Total, err = e.CountLevel(l)
	if err != nil {
		resp.Err = fmt.Sprintf("problem fetching errors: %+v", err)
		gimlet.WriteJSONError(w, resp)
		return
	}
	resp.Count = len(resp.Events)
	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /status/event/{id}

type SystemEventResponse struct {
	ID    string       `json:"id"`
	Error string       `json:"error"`
	Event *model.Event `json:"event"`
}

func (s *Service) getSystemEvent(w http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	resp := &SystemEventResponse{}
	if id == "" {
		resp.Error = "id not specified"
		gimlet.WriteJSONError(w, resp)
		return
	}
	resp.ID = id

	event := &model.Event{
		ID: id,
	}
	event.Setup(s.Environment)
	if err := event.Find(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.Event = event
	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// POST /status/event/{id}/acknowledge
//
// (nothing is read from the body)

func (s *Service) acknowledgeSystemEvent(w http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	resp := &SystemEventResponse{}
	if id == "" {
		resp.Error = "id not specified"
		gimlet.WriteJSONError(w, resp)
		return
	}
	resp.ID = id

	event := &model.Event{
		ID: id,
	}
	event.Setup(s.Environment)
	if err := event.Find(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}
	resp.Event = event

	if err := event.Acknowledge(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// POST /simple_log/{id}
//
// body: { "inc": <int>, "ts": <date>, "content": <str> }

type simpleLogRequest struct {
	Time      time.Time `json:"ts"`
	Increment int       `json:"inc"`
	Content   string    `json:"content"`
}

type SimpleLogInjestionResponse struct {
	Errors []string `json:"errors,omitempty"`
	JobID  string   `json:"jobId,omitempty"`
	LogID  string   `json:"logId"`
}

func (s *Service) simpleLogInjestion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.Environment.Context()
	defer cancel()

	req := &simpleLogRequest{}
	resp := &SimpleLogInjestionResponse{}
	resp.LogID = gimlet.GetVars(r)["id"]

	if resp.LogID == "" {
		resp.Errors = []string{"no log id specified"}
		gimlet.WriteJSONError(w, resp)
		return
	}

	if err := gimlet.GetJSON(r.Body, req); err != nil {
		grip.Error(err)
		resp.Errors = append(resp.Errors, err.Error())
		gimlet.WriteJSONError(w, resp)
		return
	}

	j := units.MakeSaveSimpleLogJob(s.Environment, resp.LogID, req.Content, req.Time, req.Increment)
	resp.JobID = j.ID()

	if err := s.queue.Put(ctx, j); err != nil {
		grip.Error(err)
		resp.Errors = append(resp.Errors, err.Error())
		gimlet.WriteJSONInternalError(w, resp)
		return
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /simple_log/{id}

type SimpleLogContentResponse struct {
	LogID string   `json:"logId"`
	Error string   `json:"err,omitempty"`
	URLS  []string `json:"urls"`
}

// simpleLogRetrieval takes in a log id and returns the log documents associated with that log id.
func (s *Service) simpleLogRetrieval(w http.ResponseWriter, r *http.Request) {
	resp := &SimpleLogContentResponse{}

	resp.LogID = gimlet.GetVars(r)["id"]
	if resp.LogID == "" {
		resp.Error = "no log specified"
		gimlet.WriteJSONError(w, resp)
		return
	}
	allLogs := &model.LogSegments{}

	if err := allLogs.Find(resp.LogID, false); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	for _, l := range allLogs.Slice() {
		resp.URLS = append(resp.URLS, l.URL)
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /simple_log/{id}/text

func (s *Service) simpleLogGetText(w http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	allLogs := &model.LogSegments{}

	if err := allLogs.Find(id, true); err != nil {
		gimlet.WriteTextError(w, err.Error())
		return
	}

	buckets := make(map[string]pail.Bucket)
	var err error
	for _, l := range allLogs.Slice() {
		bucket, ok := buckets[l.Bucket]
		if !ok {
			bucket, err = pail.NewS3Bucket(pail.S3Options{Name: l.Bucket})
			if err != nil {
				gimlet.WriteTextError(w, err.Error())
				return
			}
			buckets[l.Bucket] = bucket
		}

		func() {
			reader, err := bucket.Reader(r.Context(), l.KeyName)
			if err != nil {
				gimlet.WriteTextInternalError(w, err.Error())
				return
			}
			defer reader.Close()
			data, err := ioutil.ReadAll(reader)
			if err != nil {
				gimlet.WriteTextInternalError(w, err.Error())
				return
			}

			gimlet.WriteText(w, data)
		}()
	}
}

////////////////////////////////////////////////////////////////////////
//
// POST /system_info/
//
// body: json produced by grip/message.SystemInfo documents

type SystemInfoReceivedResponse struct {
	ID        string    `json:"id,omitempty"`
	Hostname  string    `json:"host,omitempty"`
	Timestamp time.Time `json:"time,omitempty"`
	Error     string    `json:"err,omitempty"`
}

func (s *Service) recieveSystemInfo(w http.ResponseWriter, r *http.Request) {
	resp := &SystemInfoReceivedResponse{}
	req := message.SystemInfo{}

	if err := gimlet.GetJSON(r.Body, &req); err != nil {
		grip.Error(err)
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	data := &model.SystemInformationRecord{
		Data:      req,
		Hostname:  req.Hostname,
		Timestamp: req.Time,
	}

	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	if err := data.Save(); err != nil {
		grip.Error(err)
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.ID = data.ID
	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /system_info/host/{hostname}?start=[timestamp]<,end=[timestamp],limit=[num]>

type SystemInformationResponse struct {
	Error string                `json:"error,omitempty"`
	Data  []*message.SystemInfo `json:"data"`
	Total int                   `json:"total,omitempty"`
	Limit int                   `json:"limit,omitempty"`
}

func (s *Service) fetchSystemInfo(w http.ResponseWriter, r *http.Request) {
	resp := &SystemInformationResponse{}
	host := gimlet.GetVars(r)["host"]
	if host == "" {
		resp.Error = "no host specified"
		gimlet.WriteJSONError(w, resp)
		return
	}

	startArg := r.FormValue("start")
	if startArg == "" {
		resp.Error = "no start time argument"
		gimlet.WriteJSONError(w, resp)
		return
	}

	start, err := time.Parse(time.RFC3339, startArg)
	if err != nil {
		resp.Error = fmt.Sprintf("could not parse time string '%s' in to RFC3339: %+v",
			startArg, err.Error())
		gimlet.WriteJSONError(w, resp)
		return
	}

	limitArg := r.FormValue("limit")
	if limitArg != "" {
		resp.Limit, err = strconv.Atoi(limitArg)
		if err != nil {
			resp.Error = err.Error()
			gimlet.WriteJSONError(w, resp)
			return
		}
	} else {
		resp.Limit = 100
	}

	end := time.Now()
	endArg := r.FormValue("end")
	if endArg != "" {
		end, err = time.Parse(time.RFC3339, endArg)
		if err != nil {
			resp.Error = err.Error()
			gimlet.WriteJSONError(w, resp)
			return
		}
	}

	out := &model.SystemInformationRecords{}
	count, err := out.CountHostname(host)
	if err != nil {
		resp.Error = fmt.Sprintf("could not count '%s' host: %s", host, err.Error())
		gimlet.WriteJSONError(w, resp)
		return
	}
	resp.Total = count

	err = out.FindHostnameBetween(host, start, end, resp.Limit)
	if err != nil {
		resp.Error = fmt.Sprintf("could not retrieve results, %s", err.Error())
		gimlet.WriteJSONError(w, resp)
		return
	}

	for _, d := range out.Slice() {
		resp.Data = append(resp.Data, &d.Data)
	}

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// POST /depgraph/{id}

type createDepGraphResponse struct {
	Error   string `json:"error,omitempty"`
	ID      string `json:"id,omitempty"`
	Created bool   `json:"created"`
}

func (s *Service) createDepGraph(w http.ResponseWriter, r *http.Request) {
	resp := createDepGraphResponse{}
	id := gimlet.GetVars(r)["id"]
	g := &model.GraphMetadata{
		BuildID: id,
	}

	if err := g.Find(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	if g.IsNil() {
		g.BuildID = id
		if err := g.Save(); err != nil {
			resp.Error = err.Error()
			gimlet.WriteJSONError(w, resp)
			return
		}
		resp.Created = true
	}

	resp.ID = g.BuildID
	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// GET /depgraph/{id}

type depGraphResolvedRespose struct {
	Nodes []model.GraphNode `json:"nodes"`
	Edges []model.GraphEdge `json:"edges"`
	Error string            `json:"error,omitempty"`
	ID    string            `json:"id"`
}

func (s *Service) resolveDepGraph(w http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	resp := depGraphResolvedRespose{ID: id}
	g := &model.GraphMetadata{
		BuildID: id,
	}

	if err := g.Find(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	catcher := grip.NewCatcher()

	nodes, err := g.AllNodes()
	catcher.Add(err)

	edges, err := g.AllEdges()
	catcher.Add(err)

	if catcher.HasErrors() {
		resp.Error = catcher.Resolve().Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.Edges = edges
	resp.Nodes = nodes

	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// POST /depgraph/{id}/nodes

func (s *Service) addDepGraphNodes(w http.ResponseWriter, r *http.Request) {
}

////////////////////////////////////////////////////////////////////////
//
// GET /depgraph/{id}/nodes

type depGraphNodesRespose struct {
	Nodes []model.GraphNode `json:"nodes"`
	Error string            `json:"error,omitempty"`
	ID    string            `json:"id"`
}

func (s *Service) getDepGraphNodes(w http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	resp := depGraphNodesRespose{ID: id}
	g := &model.GraphMetadata{
		BuildID: id,
	}

	if err := g.Find(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	nodes, err := g.AllNodes()
	if err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.Nodes = nodes
	gimlet.WriteJSON(w, resp)
}

////////////////////////////////////////////////////////////////////////
//
// POST /depgraph/{id}/edges

func (s *Service) addDepGraphEdges(w http.ResponseWriter, r *http.Request) {

}

////////////////////////////////////////////////////////////////////////
//
// GET /depgraph/{id}/edges

type depGraphEdgesRespose struct {
	Edges []model.GraphEdge `json:"edges"`
	Error string            `json:"error,omitempty"`
	ID    string            `json:"id"`
}

func (s *Service) getDepGraphEdges(w http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	resp := depGraphEdgesRespose{ID: id}
	g := &model.GraphMetadata{
		BuildID: id,
	}

	if err := g.Find(); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	edges, err := g.AllEdges()
	if err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.Edges = edges
	gimlet.WriteJSON(w, resp)
}

///////////////////////////////////////////////////////////////////////////////
//
// POST /admin/service/flag/{flagName}/enabled

type serviceFlagResponse struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
	State bool   `json:"state"`
}

func (s *Service) setServiceFlagEnabled(w http.ResponseWriter, r *http.Request) {
	flag := gimlet.GetVars(r)["flagName"]

	resp := serviceFlagResponse{
		Name: flag,
	}

	conf := model.NewCedarConfig(s.Environment)

	if err := conf.Flags.SetTrue(flag); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.State = true
	gimlet.WriteJSON(w, &resp)
}

func (s *Service) setServiceFlagDisabled(w http.ResponseWriter, r *http.Request) {
	flag := gimlet.GetVars(r)["flagName"]

	resp := serviceFlagResponse{
		Name: flag,
	}

	conf := model.NewCedarConfig(s.Environment)
	if err := conf.Flags.SetFalse(flag); err != nil {
		resp.Error = err.Error()
		gimlet.WriteJSONError(w, resp)
		return
	}

	resp.State = true
	gimlet.WriteJSON(w, &resp)
}

///////////////////////////////////////////////////////////////////////////////
//
// GET /admin/ca

func (s *Service) fetchRootCert(rw http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		logRequestError(r, err)
	}()

	rootcrt, err := certdepot.GetCertificate(s.Depot, s.Conf.CA.CertDepot.CAName)
	if err != nil {
		err = errors.Wrapf(err, "problem getting root certificate '%s'", s.Conf.CA.CertDepot.CAName)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}
	payload, err := rootcrt.Export()
	if err != nil {
		err = errors.Wrap(err, "problem exporting root certificate")
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(errors.Wrap(err, "problem exporting root certificate")))
		return
	}

	gimlet.WriteBinary(rw, payload)
}

///////////////////////////////////////////////////////////////////////////////
//
// POST /admin/users/key

func (s *Service) fetchUserToken(rw http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		logRequestError(r, err)
	}()

	creds := &userCredentials{}
	if err = gimlet.GetJSON(r.Body, creds); err != nil {
		err = errors.Wrap(err, "problem reading request body")
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return
	}

	if creds.Username == "" {
		err = errors.New("no user name specified")
		gimlet.WriteJSONResponse(rw, http.StatusUnauthorized, gimlet.ErrorResponse{
			Message:    "no username specified",
			StatusCode: http.StatusUnauthorized,
		})
		return
	}

	resp := &userAPIKeyResponse{Username: creds.Username}

	token, err := s.UserManager.CreateUserToken(creds.Username, creds.Password)
	if err != nil {
		err = errors.Wrapf(err, "problem creating user token for '%s'", creds.Username)
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return
	}

	user, err := s.UserManager.GetUserByToken(r.Context(), token)
	if err != nil {
		err = errors.Wrapf(err, "problem finding user '%s'", creds.Username)
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return
	}
	s.umconf.AttachCookie(token, rw)

	key := user.GetAPIKey()
	if key != "" {
		resp.Key = key
		gimlet.WriteJSON(rw, resp)
		return
	}

	dbuser, ok := user.(*model.User)
	if !ok {
		err = errors.Errorf("cannot generate key for user '%s'", creds.Username)
		gimlet.WriteJSONResponse(rw, http.StatusInternalServerError, gimlet.ErrorResponse{
			Message:    "cannot generate key for user",
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	dbuser.Setup(s.Environment)
	key, err = dbuser.SetAPIKey()
	if err != nil {
		err = errors.Errorf("problem generating key for user '%s'", creds.Username)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}

	resp.Key = key
	gimlet.WriteJSON(rw, resp)
}

///////////////////////////////////////////////////////////////////////////////
//
// POST /admin/users/certificate

func (s *Service) fetchUserCert(rw http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		logRequestError(r, err)
	}()

	usr, authorized := s.checkPayloadCreds(rw, r)
	if !authorized {
		return
	}

	opts := certdepot.CertificateOptions{
		CommonName: usr,
		CA:         s.Conf.CA.CertDepot.CAName,
		Host:       usr,
		Expires:    s.Conf.CA.SSLExpireAfter,
	}
	_, err = opts.CreateCertificateOnExpiration(s.Depot, s.Conf.CA.SSLRenewalBefore)
	if err != nil {
		err = errors.Wrapf(err, "problem updating certificate for '%s'", usr)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}

	crt, err := certdepot.GetCertificate(s.Depot, usr)
	if err != nil {
		err = errors.Wrapf(err, "problem fetching certificate for '%s'", usr)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}
	payload, err := crt.Export()
	if err != nil {
		err = errors.Wrapf(err, "problem exporting certificate for '%s'", usr)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}

	gimlet.WriteBinary(rw, payload)
}

///////////////////////////////////////////////////////////////////////////////
//
// POST /admin/users/certificate/key

func (s *Service) fetchUserCertKey(rw http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		logRequestError(r, err)
	}()

	usr, authorized := s.checkPayloadCreds(rw, r)
	if !authorized {
		return
	}

	if !certdepot.CheckPrivateKey(s.Depot, usr) {
		opts := certdepot.CertificateOptions{
			CommonName: usr,
			CA:         s.Conf.CA.CertDepot.CAName,
			Host:       usr,
			Expires:    s.Conf.CA.SSLExpireAfter,
		}
		if err = opts.CreateCertificate(s.Depot); err != nil {
			err = errors.Wrapf(err, "problem generating certificate for '%s'", usr)
			gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
			return
		}
	}

	key, err := certdepot.GetPrivateKey(s.Depot, usr)
	if err != nil {
		err = errors.Wrapf(err, "problem fetching certificate key for '%s'", usr)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}
	payload, err := key.ExportPrivate()
	if err != nil {
		err = errors.Wrapf(err, "problem exporting certificate key '%s'", usr)
		gimlet.WriteResponse(rw, gimlet.MakeJSONInternalErrorResponder(err))
		return
	}

	gimlet.WriteBinary(rw, payload)
}

///////////////////////////////////////////////////////////////////////////////
//
// helper functions

type userCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userAPIKeyResponse struct {
	Username string `json:"username"`
	Key      string `json:"key"`
}

func (s *Service) checkPayloadCreds(rw http.ResponseWriter, r *http.Request) (string, bool) {
	var err error
	defer func() {
		logRequestError(r, err)
	}()

	creds := &userCredentials{}
	if err = gimlet.GetJSON(r.Body, creds); err != nil {
		err = errors.Wrap(err, "problem reading request body")
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return "", false
	}

	if creds.Username == "" {
		err = errors.New("no username specified")
		gimlet.WriteJSONResponse(rw, http.StatusUnauthorized, gimlet.ErrorResponse{
			Message:    "no username specified",
			StatusCode: http.StatusUnauthorized,
		})
		return "", false
	}

	token, err := s.UserManager.CreateUserToken(creds.Username, creds.Password)
	if err != nil {
		err = errors.Wrapf(err, "problem creating user token for '%s'", creds.Username)
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return "", false
	}

	user, err := s.UserManager.GetUserByToken(r.Context(), token)
	if err != nil {
		err = errors.Wrapf(err, "problem finding user '%s'", creds.Username)
		gimlet.WriteResponse(rw, gimlet.MakeJSONErrorResponder(err))
		return "", false
	} else if user == nil {
		err = errors.Errorf("user '%s' not defined", creds.Username)
		gimlet.WriteJSONResponse(rw, http.StatusUnauthorized, gimlet.ErrorResponse{
			Message:    "user not defined",
			StatusCode: http.StatusUnauthorized,
		})
		return "", false
	}
	s.umconf.AttachCookie(token, rw)

	return creds.Username, true
}

func logRequestError(r *http.Request, err error) {
	grip.Error(message.WrapError(err, message.Fields{
		"method":  r.Method,
		"remote":  r.RemoteAddr,
		"request": gimlet.GetRequestID(r.Context()),
		"path":    r.URL.Path,
	}))
}
