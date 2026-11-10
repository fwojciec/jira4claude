package main_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// stubJira is the Jira site of the end-to-end tests (see e2e_test.go): an
// httptest.Server with an in-memory issue store that answers the endpoints
// the commands use, in the shapes the http package's unit tests use, plus a
// second server standing in for the media host attachment downloads redirect
// to. Both record every request.
//
// Every request to the Jira server must carry the Basic credentials the
// scratch netrc holds; any other is answered 401, as Jira would.
type stubJira struct {
	server *httptest.Server
	media  *httptest.Server

	login    string
	password string

	// pageSize caps the issues of one search page, whatever maxResults asks,
	// so that a small store still spans several pages.
	pageSize int

	// hang, when set, is a channel every Jira request waits on without
	// answering, until it is closed or the client goes away.
	hang chan struct{}

	// failReadAfterWrite makes every read of an issue that a write changed
	// fail with 503, as a read back after an applied write may.
	failReadAfterWrite bool

	// mediaStall, when set, makes the media host stall mid-body (see
	// stallMedia).
	mediaStall chan struct{}

	mu            sync.Mutex
	requests      []stubRequest
	mediaRequests []stubRequest
	issues        map[string]*stubIssue
	order         []string
	attachments   map[string]*stubAttachment
	nextIssue     int
	nextAttach    int
	nextComment   int
	nextLink      int
}

// stubRequest is one request as the stub received it.
type stubRequest struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     []byte
}

type stubIssue struct {
	Key         string
	Summary     string
	Status      string
	Type        string
	Priority    string
	Assignee    *stubUser
	Description json.RawMessage // nil: no description
	Created     string
	Updated     string
	Comments    []json.RawMessage
	Links       []json.RawMessage
	Attachments []string // IDs, in the order attached
}

type stubUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress,omitempty"`
}

type stubAttachment struct {
	ID       string
	Issue    string
	Filename string
	MimeType string
	Content  []byte
}

// The users the stub knows: the account of the credentials, and one that
// user search finds by email.
func stubMe() *stubUser {
	return &stubUser{AccountID: "acc-bob", DisplayName: "Bob Jones", Email: "bob@example.com"}
}

func stubAlice() *stubUser {
	return &stubUser{AccountID: "acc-alice", DisplayName: "Alice Smith", Email: "alice@example.com"}
}

// stubTransitions are the transitions every issue offers, as the README's
// example lists them.
const stubTransitions = `{"transitions":[` +
	`{"id":"11","name":"Start work","to":{"name":"In Progress"}},` +
	`{"id":"21","name":"Resolve","to":{"name":"Done"}},` +
	`{"id":"31","name":"Close","to":{"name":"Done"}},` +
	`{"id":"41","name":"Review","to":{"name":"In Review"}}]}`

// stubUpdatedAfterWrite is the updated time of an issue after any write.
const stubUpdatedAfterWrite = "2026-11-10T09:00:00.456+0000"

func newStubJira(t *testing.T) *stubJira {
	t.Helper()
	s := &stubJira{
		login:       "bob@example.com",
		password:    "token-" + strconv.Itoa(len(t.Name())),
		pageSize:    50,
		issues:      map[string]*stubIssue{},
		attachments: map[string]*stubAttachment{},
		nextIssue:   100,
		nextAttach:  10100,
		nextComment: 10100,
		nextLink:    20000,
	}
	s.media = httptest.NewServer(http.HandlerFunc(s.serveMedia))
	t.Cleanup(s.media.Close)
	s.server = httptest.NewServer(http.HandlerFunc(s.serveJira))
	t.Cleanup(s.server.Close)
	return s
}

// startHanging makes every later Jira request wait without an answer. It
// must be called once, before the requests: the channel it makes is closed
// by a cleanup registered after the server's own, so it runs first and
// lets Close return.
func (s *stubJira) startHanging(t *testing.T) {
	t.Helper()
	hang := make(chan struct{})
	s.mu.Lock()
	s.hang = hang
	s.mu.Unlock()
	t.Cleanup(func() { close(hang) })
}

// stallMedia makes the media host send the first half of each file, flush
// it, and then wait without sending the rest, until the client goes away
// or the test ends (the cleanup ordering is that of startHanging).
func (s *stubJira) stallMedia(t *testing.T) {
	t.Helper()
	stall := make(chan struct{})
	s.mu.Lock()
	s.mediaStall = stall
	s.mu.Unlock()
	t.Cleanup(func() { close(stall) })
}

func (s *stubJira) addIssue(issue *stubIssue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if issue.Created == "" {
		issue.Created = "2026-11-01T08:00:00.000+0000"
	}
	if issue.Type == "" {
		issue.Type = "Task"
	}
	s.issues[issue.Key] = issue
	s.order = append(s.order, issue.Key)
}

func (s *stubJira) addAttachment(a *stubAttachment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attachments[a.ID] = a
	issue := s.issues[a.Issue]
	issue.Attachments = append(issue.Attachments, a.ID)
}

// issue returns a copy of the stored issue.
func (s *stubJira) issue(key string) stubIssue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.issues[key]
}

// Requests returns the Jira requests received so far.
func (s *stubJira) Requests() []stubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubRequest(nil), s.requests...)
}

// MediaRequests returns the media host's requests received so far.
func (s *stubJira) MediaRequests() []stubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubRequest(nil), s.mediaRequests...)
}

func (s *stubJira) authorization() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(s.login+":"+s.password))
}

func record(r *http.Request) stubRequest {
	// Error ignored: a body that fails to read is recorded as far as it got.
	body, _ := io.ReadAll(r.Body)
	return stubRequest{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Header: r.Header.Clone(), Body: body}
}

func (s *stubJira) serveMedia(w http.ResponseWriter, r *http.Request) {
	req := record(r)
	s.mu.Lock()
	s.mediaRequests = append(s.mediaRequests, req)
	id := strings.TrimPrefix(r.URL.Path, "/blob/")
	a := s.attachments[id]
	stall := s.mediaStall
	s.mu.Unlock()
	if a == nil || r.URL.Query().Get("sig") != "signed-"+id {
		http.Error(w, "no such blob", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", a.MimeType)
	if stall != nil {
		w.Header().Set("Content-Length", strconv.Itoa(len(a.Content)))
		_, _ = w.Write(a.Content[:len(a.Content)/2])
		w.(http.Flusher).Flush()
		select {
		case <-stall:
		case <-r.Context().Done():
		}
		return
	}
	_, _ = w.Write(a.Content)
}

func (s *stubJira) serveJira(w http.ResponseWriter, r *http.Request) {
	req := record(r)
	s.mu.Lock()
	s.requests = append(s.requests, req)
	hang := s.hang
	s.mu.Unlock()

	if hang != nil {
		select {
		case <-hang:
		case <-r.Context().Done():
		}
		return
	}

	// Jira sets session cookies; the media host must never see them.
	http.SetCookie(w, &http.Cookie{Name: "atlassian.xsrf.token", Value: "xsrf-secret", Path: "/"})

	if r.Header.Get("Authorization") != s.authorization() {
		writeJiraError(w, http.StatusUnauthorized, "Client must be authenticated to access this resource.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	status, body := s.route(req)
	if status == http.StatusSeeOther {
		w.Header().Set("Location", string(body))
		w.WriteHeader(status)
		return
	}
	if body != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeJiraError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(jiraError(msg))
}

func jiraError(msg string) []byte {
	b, _ := json.Marshal(map[string]any{"errorMessages": []string{msg}, "errors": map[string]string{}})
	return b
}

// encode marshals v without HTML escaping, so that raw ADF passes through
// byte for byte, as Jira's own JSON does.
func encode(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// route answers one authenticated request, with s.mu held. A 303 carries
// the Location in body.
func (s *stubJira) route(req stubRequest) (int, []byte) {
	segs := strings.Split(strings.TrimPrefix(req.Path, "/rest/api/3/"), "/")
	if !strings.HasPrefix(req.Path, "/rest/api/3/") {
		return http.StatusNotFound, jiraError("stub: no route for " + req.Path)
	}
	query, _ := url.ParseQuery(req.RawQuery)
	switch {
	case req.Method == http.MethodGet && len(segs) == 1 && segs[0] == "myself":
		return http.StatusOK, encode(stubMe())

	case req.Method == http.MethodGet && len(segs) == 2 && segs[0] == "user" && segs[1] == "search":
		users := []*stubUser{}
		for _, u := range []*stubUser{stubMe(), stubAlice()} {
			if strings.EqualFold(u.Email, query.Get("query")) {
				users = append(users, u)
			}
		}
		return http.StatusOK, encode(users)

	case req.Method == http.MethodGet && len(segs) == 2 && segs[0] == "search" && segs[1] == "jql":
		return s.search(query.Get("maxResults"), query.Get("nextPageToken"))

	case req.Method == http.MethodPost && len(segs) == 1 && segs[0] == "issue":
		return s.create(req.Body)

	case req.Method == http.MethodPost && len(segs) == 1 && segs[0] == "issueLink":
		return s.link(req.Body)

	case segs[0] == "issue" && len(segs) >= 2:
		issue := s.issues[segs[1]]
		if issue == nil {
			return http.StatusNotFound, jiraError("Issue does not exist or you do not have permission to see it.")
		}
		return s.issueRoute(req, issue, segs[2:], query)

	case segs[0] == "attachment" && len(segs) == 3 && segs[1] == "content" && req.Method == http.MethodGet:
		a := s.attachments[segs[2]]
		if a == nil {
			return http.StatusNotFound, jiraError("The attachment does not exist")
		}
		// Whatever redirect=false asks, the stub sends the client to the
		// media host, the case the client must handle safely.
		return http.StatusSeeOther, []byte(s.media.URL + "/blob/" + a.ID + "?sig=signed-" + a.ID)

	case segs[0] == "attachment" && len(segs) == 2:
		a := s.attachments[segs[1]]
		if a == nil {
			return http.StatusNotFound, jiraError("The attachment does not exist")
		}
		switch req.Method {
		case http.MethodGet:
			return http.StatusOK, encode(attachmentJSON(a))
		case http.MethodDelete:
			delete(s.attachments, a.ID)
			issue := s.issues[a.Issue]
			kept := issue.Attachments[:0]
			for _, id := range issue.Attachments {
				if id != a.ID {
					kept = append(kept, id)
				}
			}
			issue.Attachments = kept
			return http.StatusNoContent, nil
		}
	}
	return http.StatusNotFound, jiraError("stub: no route for " + req.Method + " " + req.Path)
}

func (s *stubJira) issueRoute(req stubRequest, issue *stubIssue, rest []string, query url.Values) (int, []byte) {
	switch {
	case len(rest) == 0 && req.Method == http.MethodGet:
		if s.failReadAfterWrite && issue.Updated == stubUpdatedAfterWrite {
			return http.StatusServiceUnavailable, jiraError("Service unavailable")
		}
		if query.Get("fields") == "attachment" {
			return http.StatusOK, encode(map[string]any{"key": issue.Key, "fields": map[string]any{"attachment": s.attachmentField(issue)}})
		}
		return http.StatusOK, s.issueJSON(issue)

	case len(rest) == 0 && req.Method == http.MethodPut:
		var body struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil {
			return http.StatusBadRequest, jiraError("stub: bad JSON: " + err.Error())
		}
		for name, value := range body.Fields {
			switch name {
			case "description":
				if string(value) == "null" {
					issue.Description = nil
				} else {
					issue.Description = append(json.RawMessage(nil), value...)
				}
			case "summary":
				_ = json.Unmarshal(value, &issue.Summary)
			default:
				return http.StatusBadRequest, encode(map[string]any{"errorMessages": []string{}, "errors": map[string]string{name: "stub: field not supported"}})
			}
		}
		issue.Updated = stubUpdatedAfterWrite
		return http.StatusNoContent, nil

	case len(rest) == 1 && rest[0] == "transitions" && req.Method == http.MethodGet:
		return http.StatusOK, []byte(stubTransitions)

	case len(rest) == 1 && rest[0] == "transitions" && req.Method == http.MethodPost:
		var body struct {
			Transition struct {
				ID string `json:"id"`
			} `json:"transition"`
		}
		_ = json.Unmarshal(req.Body, &body)
		to := map[string]string{"11": "In Progress", "21": "Done", "31": "Done", "41": "In Review"}[body.Transition.ID]
		if to == "" {
			return http.StatusBadRequest, jiraError("Transition id '" + body.Transition.ID + "' is not valid for this issue.")
		}
		issue.Status = to
		issue.Updated = stubUpdatedAfterWrite
		return http.StatusNoContent, nil

	case len(rest) == 1 && rest[0] == "comment" && req.Method == http.MethodPost:
		var body struct {
			Body json.RawMessage `json:"body"`
		}
		if err := json.Unmarshal(req.Body, &body); err != nil || len(body.Body) == 0 {
			return http.StatusBadRequest, jiraError("stub: comment body missing")
		}
		s.nextComment++
		comment := encode(map[string]any{
			"id":      strconv.Itoa(s.nextComment),
			"author":  stubMe(),
			"body":    body.Body,
			"created": stubUpdatedAfterWrite,
		})
		issue.Comments = append(issue.Comments, comment)
		return http.StatusCreated, comment

	case len(rest) == 1 && rest[0] == "assignee" && req.Method == http.MethodPut:
		var body struct {
			AccountID *string `json:"accountId"`
		}
		_ = json.Unmarshal(req.Body, &body)
		issue.Assignee = s.user(body.AccountID)
		return http.StatusNoContent, nil

	case len(rest) == 1 && rest[0] == "attachments" && req.Method == http.MethodPost:
		return s.upload(req, issue)
	}
	return http.StatusNotFound, jiraError("stub: no route for " + req.Method + " " + req.Path)
}

func (s *stubJira) user(accountID *string) *stubUser {
	if accountID == nil {
		return nil
	}
	for _, u := range []*stubUser{stubMe(), stubAlice()} {
		if u.AccountID == *accountID {
			return u
		}
	}
	return &stubUser{AccountID: *accountID}
}

// search serves GET /search/jql: the issues in the order added, in pages of
// at most maxResults and pageSize, the next page's offset as its token. The
// JQL is recorded, not evaluated.
func (s *stubJira) search(maxResults, token string) (int, []byte) {
	limit := 50
	if maxResults != "" {
		n, err := strconv.Atoi(maxResults)
		if err != nil {
			return http.StatusBadRequest, jiraError("stub: bad maxResults")
		}
		limit = n
	}
	limit = min(limit, s.pageSize)
	start := 0
	if token != "" {
		n, err := strconv.Atoi(strings.TrimPrefix(token, "page-"))
		if err != nil {
			return http.StatusBadRequest, jiraError("stub: bad nextPageToken")
		}
		start = n
	}
	end := min(start+limit, len(s.order))
	issues := make([]json.RawMessage, 0, end-start)
	for _, key := range s.order[start:end] {
		issues = append(issues, s.issueJSON(s.issues[key]))
	}
	resp := map[string]any{"issues": issues, "isLast": end >= len(s.order)}
	if end < len(s.order) {
		resp["nextPageToken"] = "page-" + strconv.Itoa(end)
	}
	return http.StatusOK, encode(resp)
}

func (s *stubJira) create(body []byte) (int, []byte) {
	var req struct {
		Fields struct {
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
			Summary   string `json:"summary"`
			IssueType struct {
				Name string `json:"name"`
			} `json:"issuetype"`
			Description json.RawMessage `json:"description"`
			Assignee    *struct {
				ID string `json:"id"`
			} `json:"assignee"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, jiraError("stub: bad JSON")
	}
	if req.Fields.Summary == "" || req.Fields.Project.Key == "" {
		return http.StatusBadRequest, encode(map[string]any{"errorMessages": []string{}, "errors": map[string]string{"summary": "You must specify a summary of the issue."}})
	}
	s.nextIssue++
	key := req.Fields.Project.Key + "-" + strconv.Itoa(s.nextIssue)
	issue := &stubIssue{
		Key: key, Summary: req.Fields.Summary, Status: "To Do", Type: req.Fields.IssueType.Name,
		Description: req.Fields.Description, Created: stubUpdatedAfterWrite, Updated: stubUpdatedAfterWrite,
	}
	if req.Fields.Assignee != nil {
		issue.Assignee = s.user(&req.Fields.Assignee.ID)
	}
	s.issues[key] = issue
	s.order = append(s.order, key)
	return http.StatusCreated, encode(map[string]string{"id": strconv.Itoa(s.nextIssue), "key": key, "self": s.server.URL + "/rest/api/3/issue/" + key})
}

func (s *stubJira) link(body []byte) (int, []byte) {
	var req struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
		InwardIssue struct {
			Key string `json:"key"`
		} `json:"inwardIssue"`
		OutwardIssue struct {
			Key string `json:"key"`
		} `json:"outwardIssue"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, jiraError("stub: bad JSON")
	}
	in, out := s.issues[req.InwardIssue.Key], s.issues[req.OutwardIssue.Key]
	if in == nil || out == nil || req.Type.Name != "Blocks" {
		return http.StatusNotFound, jiraError("stub: unknown issue or link type")
	}
	s.nextLink++
	id := strconv.Itoa(s.nextLink)
	linkType := map[string]string{"name": "Blocks", "inward": "is blocked by", "outward": "blocks"}
	// Jira's "inwardIssue" of a link request is the issue that blocks: on
	// the inward issue the link points outward, and the other way round.
	in.Links = append(in.Links, encode(map[string]any{"id": id, "type": linkType, "outwardIssue": linkedJSON(out)}))
	out.Links = append(out.Links, encode(map[string]any{"id": id, "type": linkType, "inwardIssue": linkedJSON(in)}))
	return http.StatusCreated, nil
}

func linkedJSON(issue *stubIssue) map[string]any {
	return map[string]any{"key": issue.Key, "fields": map[string]any{
		"summary": issue.Summary, "status": map[string]string{"name": issue.Status}, "issuetype": map[string]string{"name": issue.Type},
	}}
}

// upload serves POST /issue/{key}/attachments, checking the request as
// Atlassian's OpenAPI document describes it: X-Atlassian-Token: no-check,
// a multipart/form-data body with one part named "file", and (the client's
// own promise) a Content-Length rather than chunked encoding.
func (s *stubJira) upload(req stubRequest, issue *stubIssue) (int, []byte) {
	if req.Header.Get("X-Atlassian-Token") != "no-check" {
		return http.StatusForbidden, jiraError("XSRF check failed")
	}
	if req.Header.Get("Content-Length") == "" && len(req.Body) > 0 {
		return http.StatusBadRequest, jiraError("stub: upload without Content-Length")
	}
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return http.StatusUnsupportedMediaType, jiraError("stub: not multipart/form-data")
	}
	mr := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
	var created []map[string]any
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return http.StatusBadRequest, jiraError("stub: bad multipart body: " + err.Error())
		}
		if part.FormName() != "file" {
			return http.StatusBadRequest, jiraError("stub: unexpected part " + part.FormName())
		}
		content, err := io.ReadAll(part)
		if err != nil {
			return http.StatusBadRequest, jiraError("stub: bad part")
		}
		s.nextAttach++
		a := &stubAttachment{
			ID: strconv.Itoa(s.nextAttach), Issue: issue.Key, Filename: part.FileName(),
			MimeType: mimeTypeOf(part.FileName()), Content: content,
		}
		s.attachments[a.ID] = a
		issue.Attachments = append(issue.Attachments, a.ID)
		created = append(created, attachmentJSON(a))
	}
	if len(created) != 1 {
		return http.StatusBadRequest, jiraError(fmt.Sprintf("stub: expected one file part, got %d", len(created)))
	}
	return http.StatusOK, encode(created)
}

func mimeTypeOf(name string) string {
	switch path.Ext(name) {
	case ".png":
		return "image/png"
	case ".txt", ".md":
		return "text/plain"
	}
	return "application/octet-stream"
}

// attachmentJSON is one entry of an issue's attachment field, in the shape
// of http/attachment_test.go's attachmentFieldJSON: a numeric id and Jira's
// own timestamp form.
func attachmentJSON(a *stubAttachment) map[string]any {
	id, _ := strconv.Atoi(a.ID)
	return map[string]any{
		"id": id, "filename": a.Filename, "mimeType": a.MimeType, "size": len(a.Content),
		"created": "2026-11-08T14:00:00.000+0000", "author": stubMe(),
		"content": "https://example.atlassian.net/rest/api/3/attachment/content/" + a.ID,
	}
}

func (s *stubJira) attachmentField(issue *stubIssue) []map[string]any {
	field := make([]map[string]any, 0, len(issue.Attachments))
	for _, id := range issue.Attachments {
		field = append(field, attachmentJSON(s.attachments[id]))
	}
	return field
}

// issueJSON is GET /issue/{key}: the fields the client reads, and the names
// and schema maps expand=names,schema adds.
func (s *stubJira) issueJSON(issue *stubIssue) []byte {
	var priority any
	if issue.Priority != "" {
		priority = map[string]string{"name": issue.Priority}
	}
	description := json.RawMessage("null")
	if issue.Description != nil {
		description = issue.Description
	}
	comments := append(make([]json.RawMessage, 0, len(issue.Comments)), issue.Comments...)
	links := append(make([]json.RawMessage, 0, len(issue.Links)), issue.Links...)
	return encode(map[string]any{
		"id":  strings.TrimPrefix(issue.Key, "PROJ-"),
		"key": issue.Key,
		"fields": map[string]any{
			"project":     map[string]string{"key": strings.Split(issue.Key, "-")[0]},
			"summary":     issue.Summary,
			"description": description,
			"status":      map[string]string{"name": issue.Status},
			"issuetype":   map[string]string{"name": issue.Type},
			"priority":    priority,
			"assignee":    issue.Assignee,
			"reporter":    stubMe(),
			"labels":      []string{},
			"issuelinks":  links,
			"comment":     map[string]any{"comments": comments, "total": len(comments)},
			"attachment":  s.attachmentField(issue),
			"created":     issue.Created,
			"updated":     issue.Updated,
		},
		"names":  map[string]string{},
		"schema": map[string]any{},
	})
}
