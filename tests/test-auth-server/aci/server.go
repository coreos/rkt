// Copyright 2015 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aci

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
)

type Type int

const (
	None Type = iota
	Basic
	Oauth
)

type httpError struct {
	code    int
	message string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%d: %s", e.code, e.message)
}

type serverHandler struct {
	auth  Type
	stop  chan<- struct{}
	msg   chan<- string
	tools *aciToolkit
}

func (h *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		w.WriteHeader(http.StatusOK)
		h.stop <- struct{}{}
		return
	case "GET":
		// handled later
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch h.auth {
	case None:
		// no auth to do.
	case Basic:
		payload, httpErr := getAuthPayload(r, "Basic")
		if httpErr != nil {
			w.WriteHeader(httpErr.code)
			h.sendMsg(fmt.Sprintf(`No "Authorization" header: %v`, httpErr.message))
			return
		}
		creds, err := base64.StdEncoding.DecodeString(string(payload))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			h.sendMsg(fmt.Sprintf(`Badly formed "Authorization" header`))
			return
		}
		parts := strings.Split(string(creds), ":")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			h.sendMsg(fmt.Sprintf(`Badly formed "Authorization" header (2)`))
			return
		}
		user := parts[0]
		password := parts[1]
		if user != "bar" || password != "baz" {
			w.WriteHeader(http.StatusUnauthorized)
			h.sendMsg(fmt.Sprintf("Bad credentials: %q", string(creds)))
			return
		}
	case Oauth:
		payload, httpErr := getAuthPayload(r, "Bearer")
		if httpErr != nil {
			w.WriteHeader(httpErr.code)
			h.sendMsg(fmt.Sprintf(`No "Authorization" header: %v`, httpErr.message))
			return
		}
		if payload != "sometoken" {
			w.WriteHeader(http.StatusUnauthorized)
			h.sendMsg(fmt.Sprintf(`Bad token: %q`, payload))
			return
		}
	default:
		panic("Woe is me!")
	}
	h.sendMsg(fmt.Sprintf("Trying to serve %q", r.URL.String()))
	switch filepath.Base(r.URL.Path) {
	case "prog.aci":
		h.sendMsg(fmt.Sprintf("  serving"))
		if data, err := h.tools.prepareACI(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.sendMsg(fmt.Sprintf("    failed (%v)", err))
		} else {
			w.Write(data)
			h.sendMsg(fmt.Sprintf("    done."))
		}
	default:
		h.sendMsg(fmt.Sprintf("  not found."))
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *serverHandler) sendMsg(msg string) {
	select {
	case h.msg <- msg:
	default:
	}
}

func getAuthPayload(r *http.Request, authType string) (string, *httpError) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		err := &httpError{
			code:    http.StatusUnauthorized,
			message: "No auth",
		}
		return "", err
	}
	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		err := &httpError{
			code:    http.StatusBadRequest,
			message: "Malformed auth",
		}
		return "", err
	}
	if parts[0] != authType {
		err := &httpError{
			code:    http.StatusUnauthorized,
			message: "Wrong auth",
		}
		return "", err
	}
	return parts[1], nil
}

type Server struct {
	Stop    <-chan struct{}
	Msg     <-chan string
	Conf    string
	URL     string
	handler *serverHandler
	http    *httptest.Server
}

func (s *Server) Close() {
	s.http.Close()
	close(s.handler.msg)
	close(s.handler.stop)
}

func NewServer(auth Type, msgCapacity int) (*Server, error) {
	return NewServerWithPaths(auth, msgCapacity, "actool", "go")
}

func NewServerWithPaths(auth Type, msgCapacity int, acTool, goTool string) (*Server, error) {
	if !filepath.IsAbs(acTool) {
		absAcTool, err := getTool(acTool)
		if err != nil {
			return nil, err
		}
		acTool = absAcTool
	}
	if !filepath.IsAbs(goTool) {
		absGoTool, err := getTool(goTool)
		if err != nil {
			return nil, err
		}
		goTool = absGoTool
	}
	stop := make(chan struct{})
	msg := make(chan string, msgCapacity)
	server := &Server{
		Stop: stop,
		Msg:  msg,
		handler: &serverHandler{
			auth: auth,
			stop: stop,
			msg:  msg,
			tools: &aciToolkit{
				acTool: acTool,
				goTool: goTool,
			},
		},
	}
	server.http = httptest.NewUnstartedServer(server.handler)
	server.http.TLS = &tls.Config{InsecureSkipVerify: true}
	server.http.StartTLS()
	server.URL = server.http.URL
	host := server.http.Listener.Addr().String()
	switch auth {
	case None:
		// nothing to do
	case Basic:
		creds := `"user": "bar",
		"password": "baz"`
		server.Conf = sprintCreds(host, "basic", creds)
	case Oauth:
		creds := `"token": "sometoken"`
		server.Conf = sprintCreds(host, "oauth", creds)
	default:
		panic("Woe is me!")
	}
	return server, nil
}

func getTool(tool string) (string, error) {
	toolPath, err := exec.LookPath(tool)
	if err != nil {
		return "", fmt.Errorf("failed to find %s in $PATH: %v", tool, err)
	}
	absToolPath, err := filepath.Abs(toolPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of %s: %v", tool, err)
	}
	return absToolPath, nil
}

func sprintCreds(host, auth, creds string) string {
	return fmt.Sprintf(`
{
	"rktKind": "auth",
	"rktVersion": "v1",
	"domains": ["%s"],
	"type": "%s",
	"credentials":
	{
		%s
	}
}

`, host, auth, creds)
}
